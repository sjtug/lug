package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/manager"
)

const (
	LugVersionInfo = `Lug: An extensible backend for software mirror
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
      name: putty`
)

// Store parsed flags from command line
type CommandFlags struct {
	configFile string
	version    bool
}

// parse command line options and return CommandFlags
func getFlags() (flags CommandFlags) {
	flag.StringVar(&flags.configFile, "c", "config.yaml",
		configHelp)
	flag.BoolVar(&flags.version, "v", false, "Prints version of lug")
	flag.Parse()
	return
}

// Register Logger and set logLevel
func prepareLogger(logLevel logging.Level) {
	baseLogger := logging.NewLogBackend(os.Stdout, "", 0)
	logger := logging.AddModuleLevel(baseLogger)
	logger.SetLevel(logLevel, "")
	logging.SetBackend(logger)
}

func main() {
	flags := getFlags()

	if flags.version {
		fmt.Print(LugVersionInfo)
		return
	}

	dat, err := ioutil.ReadFile(flags.configFile)
	if err != nil {
		panic(err)
	}

	cfg := config.Config{}
	err = cfg.Parse(dat)
	prepareLogger(cfg.LogLevel)

	curLogger := logging.MustGetLogger("lug")
	curLogger.Info("Starting...")
	curLogger.Debugf("%+v\n", cfg)
	if err != nil {
		panic(err)
	}

	m, err := manager.NewManager(&cfg)
	if err != nil {
		panic(err)
	}
	m.Run()

}
