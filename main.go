package main

import (
	"flag"
	"io/ioutil"
	"os"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/manager"
)

type CommandFlags struct {
	configFile string
}

func getFlags() (flags CommandFlags) {
	flag.StringVar(&flags.configFile, "c", "config.yaml",
		`Configuration file of lug.
	Example:
	interval: 30 # Interval between pollings
	loglevel: 5 # 1-5
	repos:
	 - type: rsync
	   source: vim.org
	   interval: 600 # Interval between sync
	`)
	flag.Parse()
	return
}

func main() {
	flags := getFlags()
	dat, err := ioutil.ReadFile(flags.configFile)
	if err != nil {
		panic(err)
	}

	cfg := config.Config{}
	err = cfg.Parse(dat)
	baseLogger := logging.NewLogBackend(os.Stdout, "", 0)

	logger := logging.AddModuleLevel(baseLogger)
	logger.SetLevel(cfg.LogLevel, "")
	logging.SetBackend(logger)

	curLogger := logging.MustGetLogger("lug")
	curLogger.Info("Starting...")
	curLogger.Debugf("%+v\n", cfg)
	if err != nil {
		panic(err)
	}

	m := manager.NewManager(&cfg, curLogger)
	m.Run()

}
