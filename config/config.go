// Package config provides the definition of Config and a method
// to parse it from a []byte
package config

import (
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
	// LogLevel: 1-5 is acceptable
	LogLevel logging.Level
	// Config for each repo is represented as an array of RepoConfig
	Repos []RepoConfig
}

// Function to parse config from []byte
func (c *Config) Parse(in []byte) error {
	return yaml.Unmarshal(in, c)
}

func Foo() {
	fmt.Println("config")
}
