package main

import (
	"fmt"
	"net/http"
	"os"
	osexec "os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/cheshir/logrustash"
	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"

	"github.com/sjtug/lug/pkg/config"
	"github.com/sjtug/lug/pkg/exporter"
	"github.com/sjtug/lug/pkg/manager"
)

const (
	lugVersionInfo = `Lug: An extensible backend for software mirror
	Presented by SJTUG Version 0.12.1

Visit https://github.com/sjtug/lug for latest version`
	configHelp = `Refer to config.example.yaml for sample config!`
	// rlimitHelperCommand is intentionally hidden. Worker subprocesses use it
	// to apply rlimit_mem to the child command without mutating the long-lived
	// Lug daemon process.
	rlimitHelperCommand = "__run-with-rlimit"
)

// CommandFlags stores parsed flags from command line
type CommandFlags struct {
	configFile   string
	version      bool
	license      bool
	jsonAPIAddr  string
	exporterAddr string
}

// parse command line options and return CommandFlags
func getFlags() (flags CommandFlags) {
	flag.StringVarP(&flags.configFile, "conf", "c", "config.yaml",
		configHelp)
	flag.BoolVar(&flags.license, "license", false, "Prints license of used libraries")
	flag.BoolVarP(&flags.version, "version", "v", false, "Prints version of lug")
	flag.StringVarP(&flags.jsonAPIAddr, "jsonapi", "j", "", "JSON API Address")
	flag.StringVarP(&flags.exporterAddr, "exporter", "e", "", "Exporter Address")
	flag.Parse()
	return
}

// Register Logger and set logLevel
func prepareLogger(logLevel log.Level, logStashAddr string, additionalFields map[string]interface{}) {
	log.SetLevel(logLevel)

	// 12-factor XI: logs are an event stream written unbuffered to stdout.
	// Docker's logging driver (journald/loki/json-file) handles routing.
	log.SetFormatter(&log.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
		FieldMap: log.FieldMap{
			log.FieldKeyMsg: "msg",
		},
	})
	log.SetOutput(os.Stdout)

	if logStashAddr != "" {
		hook, err := logrustash.NewAsyncHookWithFields("tcp", logStashAddr, "lug", additionalFields)
		if err != nil {
			// A misconfigured logstash sink must not take the scheduler down;
			// the stdout event stream remains the source of truth.
			log.WithError(err).Warn("logstash hook disabled")
		} else {
			hook.WaitUntilBufferFrees = true
			hook.ReconnectBaseDelay = time.Second
			hook.ReconnectDelayMultiplier = 2
			hook.MaxSendRetries = 10
			log.AddHook(hook)
		}
	}
}

func loadConfigFromFlags() config.Config {
	flags := getFlags()

	cfgViper := config.CfgViper
	_ = cfgViper.BindPFlag("json_api.address", flag.Lookup("jsonapi"))
	_ = cfgViper.BindPFlag("exporter_address", flag.Lookup("exporter"))

	if flags.version {
		fmt.Print(lugVersionInfo)
		os.Exit(0)
	}

	if flags.license {
		fmt.Print(licenseText)
		os.Exit(0)
	}

	file, err := os.Open(flags.configFile)
	if err != nil {
		log.Error(err)
		fmt.Print(configHelp)
		os.Exit(0)
	}
	defer func() { _ = file.Close() }()
	cfg := config.Config{}
	err = cfg.Parse(file)

	prepareLogger(cfg.LogLevel, cfg.LogStashConfig.Address, cfg.LogStashConfig.AdditionalFields)
	log.Info("Starting...")
	log.Debugln(spew.Sdump(cfg))
	if err != nil {
		panic(err)
	}
	return cfg
}

func runRlimitHelper(args []string) int {
	if len(args) < 3 || args[1] != "--" {
		fmt.Fprintf(os.Stderr, "usage: %s <limit> -- <command> [args...]\n", rlimitHelperCommand)
		return 2
	}
	limitBytes, err := humanize.ParseBytes(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid rlimit_mem %q: %v\n", args[0], err)
		return 2
	}

	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &limit); err != nil {
		fmt.Fprintf(os.Stderr, "getrlimit(RLIMIT_AS): %v\n", err)
		return 1
	}
	limit.Cur = limitBytes
	if err := syscall.Setrlimit(syscall.RLIMIT_AS, &limit); err != nil {
		fmt.Fprintf(os.Stderr, "setrlimit(RLIMIT_AS=%s): %v\n", args[0], err)
		return 1
	}

	cmdArgs := append([]string(nil), args[2:]...)
	cmdPath, err := osexec.LookPath(cmdArgs[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "lookup %q: %v\n", cmdArgs[0], err)
		return 127
	}
	cmdArgs[0] = cmdPath
	if err := syscall.Exec(cmdPath, cmdArgs, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "exec %q: %v\n", cmdPath, err)
		return 127
	}
	return 127
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == rlimitHelperCommand {
		os.Exit(runRlimitHelper(os.Args[2:]))
	}

	cfg := loadConfigFromFlags()
	m, err := manager.NewManager(&cfg)
	if err != nil {
		panic(err)
	}

	// 12-factor IX: dispose on SIGINT/SIGTERM. The cancellable context is
	// propagated to workers so in-flight rsync children (exec.CommandContext)
	// are killed rather than orphaned, and m.Run() returns so main exits 0.
	m.InstallShutdownContext()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.WithField("event", "signal_received").WithField("signal", sig.String()).Info("shutting down")
		m.Shutdown()
	}()

	jsonapi := manager.NewRestfulAPI(m)
	handler := jsonapi.GetAPIHandler()
	go func() {
		if err := http.ListenAndServe(cfg.JsonAPIConfig.Address, handler); err != nil {
			log.Error(err)
		}
	}()

	go exporter.Expose(cfg.ExporterAddr)
	m.Run()
	log.WithField("event", "exited").Info("lug exited")
}
