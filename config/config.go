package config

import (
	"fmt"

	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
)

type RepoConfig map[string]string

type Config struct {
	Interval int
	LogLevel logging.Level
	Repos    []RepoConfig
}

func (c *Config) Parse(in []byte) error {
	return yaml.Unmarshal(in, c)
}

func Foo() {
	fmt.Println("config")
}
