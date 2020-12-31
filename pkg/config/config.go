// Package config provides the definition of Config and a method
// to parse it from a []byte
package config

import (
	"errors"
	"io"
	"reflect"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// RepoConfig stores config of each repo in a map
type RepoConfig map[string]interface{}

type JsonAPIConfig struct {
	// The address that lug listens for JSON API
	Address string
}

type LogStashConfig struct {
	Address          string
	AdditionalFields map[string]interface{} `mapstructure:"additional_fields"`
}

// Config stores all configuration of lug
type Config struct {
	// Interval between pollings in manager
	Interval int
	// LogLevel: 0-5 is acceptable
	LogLevel log.Level
	// ConcurrentLimit: how many worker can run at the same time
	ConcurrentLimit int `mapstructure:"concurrent_limit"`
	// LogStashConfig represents configurations for logstash
	LogStashConfig LogStashConfig `mapstructure:"logstash"`
	// ExporterAddr is the address to expose metrics, :8080 for default
	ExporterAddr string `mapstructure:"exporter_address"`
	// JsonAPIConfig specifies configuration of JSON restful API
	JsonAPIConfig JsonAPIConfig `mapstructure:"json_api"`
	// Worker sync checkpoint path
	Checkpoint string `mapstructure:"checkpoint"`
	// Config for each repo is represented as an array of RepoConfig. Nested structure is disallowed
	Repos []RepoConfig
	// A dummy section that will not be used in our program.
	Dummy interface{} `mapstructure:"dummy"`
}

// CfgViper is the instance of config
var CfgViper *viper.Viper

func init() {
	CfgViper = viper.New()
	CfgViper.SetDefault("loglevel", 4)
	CfgViper.SetDefault("json_api.address", ":7001")
	CfgViper.SetDefault("exporter_address", ":8080")
	CfgViper.SetDefault("concurrent_limit", 5)
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
		if c.ConcurrentLimit <= 0 {
			return errors.New("concurrent limit must be positive")
		}
	}
	for _, repo := range c.Repos {
		for _, v := range repo {
			t := reflect.TypeOf(v)
			if t == nil {
				continue
			}
			kind := t.Kind()
			var invalidKinds = map[reflect.Kind]bool{
				reflect.Array:     true,
				reflect.Map:       true,
				reflect.Slice:     true,
				reflect.Struct:    true,
				reflect.Interface: true,
			}
			if _, ok := invalidKinds[kind]; ok {
				return errors.New("nested property(e.g. arrays/maps) in Repos is disallowed: " + spew.Sdump(v))
			}
		}
	}
	return err
}
