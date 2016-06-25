// Package config provides the definition of Config and a method
// to parse it from a []byte
package config

import (
	"errors"
	"fmt"

	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
)

// Config of each repo is represented as a map
type RepoConfig map[string]string

// Configuration of lug
type Config struct {
	// Interval between pollings in manager
	Interval int
	// LogLevel: 0-5 is acceptable
	LogLevel logging.Level
	// Config for each repo is represented as an array of RepoConfig
	Repos []RepoConfig
}

// Function to parse config from []byte
func (c *Config) Parse(in []byte) (err error) {
	err = yaml.Unmarshal(in, c)
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

func Foo() {
	fmt.Println("config")
}
