package main

import (
	"fmt"
	flag "github.com/spf13/pflag"
	"net/http"

	"github.com/cheshir/logrustash"
	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/exporter"
	"github.com/sjtug/lug/manager"
	"os"
	"time"
)

const (
	lugVersionInfo = `Lug: An extensible backend for software mirror
	Presented by SJTUG Version 0.10.0
	
Visit https://github.com/sjtug/lug for latest version`
	configHelp = `Refer to config.example.yaml for sample config!`
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

	go exporter.Expose(cfg.ExporterAddr)
	m.Run()
}
