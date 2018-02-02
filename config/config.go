// Package config provides the definition of Config and a method
// to parse it from a []byte
package config

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// RepoConfig stores config of each repo in a map
type RepoConfig map[string]string

// Config stores all configuration of lug
type Config struct {
	// Interval between pollings in manager
	Interval int
	// LogLevel: 0-5 is acceptable
	LogLevel log.Level
	// LogStashAddr represents the address of logstash
	LogStashAddr string
	// ExporterAddr is the address to expose metrics, :8080 for default
	ExporterAddr string `yaml:"exporter_address"`
	// Config for each repo is represented as an array of RepoConfig
	Repos []RepoConfig
	// FileServeAddr represents at which address/port files will be served
	FileServeAddr string `yaml:"file_serve_address"`
	// FileLogPath specifies where served files' log will be stored
	FileLogPath string `yaml:"file_log_path"`
}

// Parse creates config from []byte
func (c *Config) Parse(in []byte) (err error) {
	err = yaml.Unmarshal(in, c)
	if err == nil {
		if c.Interval < 0 {
			return errors.New("Interval can't be negative")
		}
		if c.LogLevel < 0 || c.LogLevel > 5 {
			return errors.New("loglevel must be 0-5")
		}
		if c.ExporterAddr == "" {
			c.ExporterAddr = ":8080"
		}
		if c.FileServeAddr == "" {
			c.FileServeAddr = ":9000"
		}
		if c.FileLogPath == "" {
			c.FileLogPath = "file-access.log"
		}
	}
	return err
}
