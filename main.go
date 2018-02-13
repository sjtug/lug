package main

import (
	"fmt"
	flag "github.com/spf13/pflag"
	"net/http"

	"github.com/bshuster-repo/logrus-logstash-hook"
	"github.com/davecgh/go-spew/spew"
	"github.com/goji/httpauth"
	log "github.com/sirupsen/logrus"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/exporter"
	"github.com/sjtug/lug/manager"
	"os"
)

const (
	lugVersionInfo = `Lug: An extensible backend for software mirror
	Presented by SJTUG Version 0.6.0
	
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
	certFile     string
	keyFile      string
	apiUser      string
	apiPassword  string
}

// parse command line options and return CommandFlags
func getFlags() (flags CommandFlags) {
	flag.StringVarP(&flags.configFile, "conf", "c", "config.yaml",
		configHelp)
	flag.BoolVar(&flags.license, "license", false, "Prints license of used libraries")
	flag.BoolVarP(&flags.version, "version", "v", false, "Prints version of lug")
	flag.StringVarP(&flags.jsonAPIAddr, "jsonapi", "j", "", "JSON API Address")
	flag.StringVarP(&flags.exporterAddr, "exporter", "e", "", "Exporter Address")
	flag.StringVar(&flags.certFile, "cert", "", "HTTPS Cert file of JSON API")
	flag.StringVar(&flags.keyFile, "key", "", "HTTPS Key file of JSON API")
	flag.StringVarP(&flags.apiUser, "api-user", "u", "", "User for authentication of JSON API")
	flag.StringVarP(&flags.apiPassword, "api-password", "p", "", "Password for authentication of JSON API")
	flag.Parse()
	return
}

// Register Logger and set logLevel
func prepareLogger(logLevel log.Level, logStashAddr string) {
	log.SetLevel(logLevel)
	if logStashAddr != "" {
		hook, err := logrus_logstash.NewHook("tcp", logStashAddr, "lug")
		if err != nil {
			log.Fatal(err)
		}
		log.AddHook(hook)
	}
}

var cfg config.Config

func init() {
	flags := getFlags()

	cfgViper := config.CfgViper
	cfgViper.BindPFlag("json_api.address", flag.Lookup("jsonapi"))
	cfgViper.BindPFlag("json_api.certfile", flag.Lookup("cert"))
	cfgViper.BindPFlag("json_api.keyfile", flag.Lookup("key"))
	cfgViper.BindPFlag("json_api.username", flag.Lookup("api-user"))
	cfgViper.BindPFlag("json_api.password", flag.Lookup("api-password"))
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

	prepareLogger(cfg.LogLevel, cfg.LogStashAddr)
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
	if cfg.JsonAPIConfig.Username != "" && cfg.JsonAPIConfig.Password != "" {
		auth := httpauth.BasicAuth(httpauth.AuthOptions{
			Realm:    "Require authentication",
			User:     cfg.JsonAPIConfig.Username,
			Password: cfg.JsonAPIConfig.Password,
		})
		handler = auth(handler)
	}
	if cfg.JsonAPIConfig.KeyFile == "" || cfg.JsonAPIConfig.CertFile == "" {
		if cfg.JsonAPIConfig.Username != "" && cfg.JsonAPIConfig.Password != "" {
			log.Warn("JSON API with HTTP auth without TLS/SSL is vulnerable")
		}
		log.Infof("Http JSON API listening on %s", cfg.JsonAPIConfig.Address)
		go http.ListenAndServe(cfg.JsonAPIConfig.Address, handler)
	} else {
		log.Infof("Https JSON API listening on %s with certfile %s and keyfile %s", cfg.JsonAPIConfig.Address,
			cfg.JsonAPIConfig.CertFile, cfg.JsonAPIConfig.KeyFile)
		go http.ListenAndServeTLS(cfg.JsonAPIConfig.Address, cfg.JsonAPIConfig.CertFile, cfg.JsonAPIConfig.KeyFile, handler)
	}

	go exporter.Expose(cfg.ExporterAddr)
	m.Run()
}
