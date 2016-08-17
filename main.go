package main

import (
	"flag"
	"fmt"
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
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
func prepareLogger(logLevel log.Level) {
	log.SetLevel(logLevel)
}

func main() {
	flags := getFlags()

	if flags.version {
		fmt.Print(lugVersionInfo)
		return
	}

	dat, err := ioutil.ReadFile(flags.configFile)
	if err != nil {
		panic(err)
	}

	cfg := config.Config{}
	err = cfg.Parse(dat)
	prepareLogger(cfg.LogLevel)

	log.Info("Starting...")
	log.Debugf("%+v\n", cfg)
	if err != nil {
		panic(err)
	}

	m, err := manager.NewManager(&cfg)
	if err != nil {
		panic(err)
	}
	m.Run()

}
