package config

import (
	"sync"

	"github.com/kelseyhightower/envconfig"
)

// prefix is the app config env var prefix
const prefix = ""

// Config contains configuration parameters
type Config struct {
	DuckDNS DuckDNS `envconfig:"duckdns"`
}

// DuckDNS represents the configuration settings required to interact with the DuckDNS service.
type DuckDNS struct {
	Protocol string `envconfig:"protocol" default:"https"`
	Domain   string `envconfig:"domain" default:"www.duckdns.org"`
	Verbose  bool   `envconfig:"verbose" default:"true"`
}

// MustGet returns config after initializing it or panics
func MustGet() *Config {
	c, err := initialize()
	if err != nil {
		panic(err)
	}
	return c
}

// Get returns config after initializing it or errors if it fails to initialize
func Get() (*Config, error) {
	c, err := initialize()
	if err != nil {
		return nil, err
	}
	return c, nil
}

// global app configuration instance
var c *Config
var mutex sync.Mutex

// initialize app configuration
// this initializes the configuration only once if not already initialized
func initialize() (*Config, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if c != nil {
		return c, nil
	}
	c = new(Config)
	if err := envconfig.Process(prefix, c); err != nil {
		return nil, err
	}
	return c, nil
}
