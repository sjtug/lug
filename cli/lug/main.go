package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/cheshir/logrustash"
	"github.com/davecgh/go-spew/spew"
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
)

// CommandFlags stores parsed flags from command line
type CommandFlags struct {
	configFile   string
	syncRepo     string
	version      bool
	license      bool
	jsonAPIAddr  string
	exporterAddr string
}

// parse command line options and return CommandFlags
func getFlags() (flags CommandFlags) {
	flag.StringVarP(&flags.configFile, "conf", "c", "config.yaml",
		configHelp)
	flag.StringVarP(&flags.syncRepo, "sync", "s", "", "manual sync repo")
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
	if logStashAddr != "" {
		hook, err := logrustash.NewAsyncHookWithFields("tcp", logStashAddr, "lug", additionalFields)
		if err != nil {
			log.Fatal(err)
		}
		hook.WaitUntilBufferFrees = true
		hook.ReconnectBaseDelay = time.Second
		hook.ReconnectDelayMultiplier = 2
		hook.MaxSendRetries = 10
		log.AddHook(hook)
	}
}

var cfg config.Config
var manualSyncRepo string

func init() {
	flags := getFlags()

	cfgViper := config.CfgViper
	cfgViper.BindPFlag("json_api.address", flag.Lookup("jsonapi"))
	cfgViper.BindPFlag("exporter_address", flag.Lookup("exporter"))

	if flags.version {
		fmt.Print(lugVersionInfo)
		os.Exit(0)
	}

	if flags.license {
		fmt.Print(licenseText)
		os.Exit(0)
	}

	manualSyncRepo = flags.syncRepo

	file, err := os.Open(flags.configFile)
	if err != nil {
		log.Error(err)
		fmt.Print(configHelp)
		os.Exit(0)
	}
	defer file.Close()
	cfg = config.Config{}
	err = cfg.Parse(file)

	prepareLogger(cfg.LogLevel, cfg.LogStashConfig.Address, cfg.LogStashConfig.AdditionalFields)
	log.Info("Starting...")
	log.Debugln(spew.Sdump(cfg))
	if err != nil {
		panic(err)
	}
}

func main() {
	m, err := manager.NewManager(&cfg)
	if err != nil {
		panic(err)
	}
	jsonapi := manager.NewRestfulAPI(m)
	handler := jsonapi.GetAPIHandler()
	go http.ListenAndServe(cfg.JsonAPIConfig.Address, handler)

	if manualSyncRepo == "" {
		go exporter.Expose(cfg.ExporterAddr)
		m.Run()
	} else {
		m.RunSpecificWorker(manualSyncRepo)
	}
}
