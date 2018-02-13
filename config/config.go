// Package config provides the definition of Config and a method
// to parse it from a []byte
package config

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
)

// RepoConfig stores config of each repo in a map
type RepoConfig map[string]string

type JsonAPIConfig struct {
	// The address that lug listens for JSON API
	Address string
	// HTTP basic auth username
	Username string
	// HTTP basic auth password
	Password string
	// Https certfile
	CertFile string
	// Https keyfile
	KeyFile string
}

// Config stores all configuration of lug
type Config struct {
	// Interval between pollings in manager
	Interval int
	// LogLevel: 0-5 is acceptable
	LogLevel log.Level
	// LogStashAddr represents the address of logstash
	LogStashAddr string `mapstructure:"logstash_address"`
	// ExporterAddr is the address to expose metrics, :8080 for default
	ExporterAddr string `mapstructure:"exporter_address"`
	// JsonAPIConfig specifies configuration of JSON restful API
	JsonAPIConfig JsonAPIConfig `mapstructure:"json_api"`
	// Config for each repo is represented as an array of RepoConfig
	Repos []RepoConfig
}

// CfgViper is the instance of config
var CfgViper *viper.Viper

func init() {
	CfgViper = viper.New()
	CfgViper.SetDefault("loglevel", 4)
	CfgViper.SetDefault("json_api.address", ":7001")
	CfgViper.SetDefault("exporter_address", ":8080")
}

// Parse creates config from a reader
func (c *Config) Parse(in io.Reader) (err error) {
	CfgViper.SetConfigType("yaml")
	err = CfgViper.ReadConfig(in)
	if err != nil {
		return err
	}
	err = CfgViper.UnmarshalExact(&c)
	if err == nil {
		if c.Interval < 0 {
			return errors.New("Interval can't be negative")
		}
		if c.LogLevel < 0 || c.LogLevel > 5 {
			return errors.New("loglevel must be 0-5")
		}
	}
	return err
}
