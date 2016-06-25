package main

import (
	"flag"
	"io/ioutil"

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

}

func main() {
	flags := getFlags()
	dat, err := ioutil.ReadFile(flags.configFile)
	if err != nil {
		panic(err)
	}

	cfg := config.Config{}
	cfg.Parse(dat)

	baseLogger := logging.NewLogBackend(os.Stderr, "", 0)

	// Only errors and more severe messages should be sent to backend1
	logger := logging.AddModuleLevel(baseLogger)
	logger.SetLevel(cfg.LogLevel, "")
	logging.SetBackend(logger)
	m := manager.NewManager(&cfg, logger)
	m.Run()

}
