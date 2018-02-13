package main

import (
	"fmt"
	flag "github.com/spf13/pflag"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/bshuster-repo/logrus-logstash-hook"
	"github.com/goji/httpauth"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/exporter"
	"github.com/sjtug/lug/manager"
)

const (
	lugVersionInfo = `Lug: An extensible backend for software mirror
	Presented by SJTUG Version 0.5.0
	
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
	flag.StringVarP(&flags.jsonAPIAddr, "jsonapi", "j", ":7001", "JSON API Address")
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
var flags CommandFlags

func init() {
	flags = getFlags()

	if flags.version {
		fmt.Print(lugVersionInfo)
		return
	}

	if flags.license {
		fmt.Print(licenseText)
		return
	}

	dat, err := ioutil.ReadFile(flags.configFile)
	if err != nil {
		log.Error(err)
		fmt.Print(configHelp)
		return
	}

	cfg = config.Config{}
	err = cfg.Parse(dat)
	prepareLogger(cfg.LogLevel, cfg.LogStashAddr)
	log.Info("Starting...")
	log.Debugf("%+v\n", cfg)
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
	if flags.apiUser != "" && flags.apiPassword != "" {
		auth := httpauth.BasicAuth(httpauth.AuthOptions{
			Realm:    "Require authentication",
			User:     flags.apiUser,
			Password: flags.apiPassword,
		})
		handler = auth(handler)
	}
	if flags.keyFile == "" || flags.certFile == "" {
		if flags.apiUser != "" && flags.apiPassword != "" {
			log.Warn("JSON API with HTTP auth without TLS/SSL is vulnerable")
		}
		log.Infof("Http JSON API listening on %s", flags.jsonAPIAddr)
		go http.ListenAndServe(flags.jsonAPIAddr, handler)
	} else {
		log.Infof("Https JSON API listening on %s with certfile %s and keyfile %s", flags.jsonAPIAddr,
			flags.certFile, flags.keyFile)
		go http.ListenAndServeTLS(flags.jsonAPIAddr, flags.certFile, flags.keyFile, handler)
	}

	if flags.exporterAddr != "" {
		cfg.ExporterAddr = flags.exporterAddr
	}
	go exporter.Expose(cfg.ExporterAddr)
	m.Run()
}
