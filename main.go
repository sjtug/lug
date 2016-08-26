package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/bshuster-repo/logrus-logstash-hook"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/manager"
)

const (
	lugVersionInfo = `Lug: An extensible backend for software mirror
	Presented by SJTUG Version 0.1alpha
	
Visit https://github.com/sjtug/lug for latest version`
	configHelp = `Configuration file of lug.
Example:
interval: 3 # Interval between pollings
loglevel: 5 # 1-5
repos:
    - type: rsync
      source: rsync://rsync.chiark.greenend.org.uk/ftp/users/sgtatham/putty-website-mirror/
      interval: 6
      path: /tmp/putty
      name: putty
      rlimit_mem: 200M
    - type: shell_script
      script: /path/to/your/script
      interval: 6
      name: shell`
)

// CommandFlags stores parsed flags from command line
type CommandFlags struct {
	configFile  string
	version     bool
	license     bool
	jsonAPIAddr string
	certFile    string
	keyFile     string
}

// parse command line options and return CommandFlags
func getFlags() (flags CommandFlags) {
	flag.StringVar(&flags.configFile, "c", "config.yaml",
		configHelp)
	flag.BoolVar(&flags.license, "license", false, "Prints license of used libraries")
	flag.BoolVar(&flags.version, "v", false, "Prints version of lug")
	flag.StringVar(&flags.jsonAPIAddr, "j", ":7001", "JSON API Address")
	flag.StringVar(&flags.certFile, "cert", "", "HTTPS Cert file of JSON API")
	flag.StringVar(&flags.keyFile, "key", "", "HTTPS Key file of JSON API")
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

func main() {
	flags := getFlags()

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

	cfg := config.Config{}
	err = cfg.Parse(dat)
	prepareLogger(cfg.LogLevel, cfg.LogStashAddr)

	log.Info("Starting...")
	log.Debugf("%+v\n", cfg)
	if err != nil {
		panic(err)
	}

	m, err := manager.NewManager(&cfg)
	if err != nil {
		panic(err)
	}
	jsonapi := manager.NewRestfulAPI(m)
	if flags.keyFile == "" || flags.certFile == "" {
		log.Infof("Http JSON API listening on %s", flags.jsonAPIAddr)
		go http.ListenAndServe(flags.jsonAPIAddr, jsonapi.GetAPIHandler())
	} else {
		log.Infof("Https JSON API listening on %s with certfile %s and keyfile %s", flags.jsonAPIAddr,
			flags.certFile, flags.keyFile)
		go http.ListenAndServeTLS(flags.jsonAPIAddr, flags.certFile, flags.keyFile, jsonapi.GetAPIHandler())
	}

	m.Run()

}
