package config

import (
	"sync"

	"github.com/kelseyhightower/envconfig"
)

// prefix is the app config env var prefix
const prefix = ""

// Config contains configuration parameters for the DNS synchronizer CronJob application.
type Config struct {
	DuckDNS   DuckDNS   `envconfig:"duckdns"`
	DNSRecord DNSRecord `envconfig:"dnsrecord"`
}

// DuckDNS represents the configuration settings required to interact with the DuckDNS service.
type DuckDNS struct {
	Protocol string `envconfig:"protocol" default:"https"`
	Domain   string `envconfig:"domain" default:"www.duckdns.org"`
	Verbose  bool   `envconfig:"verbose" default:"false"`
}

// DNSRecord represents the configuration settings required to manage a DNS record in Kubernetes.
type DNSRecord struct {
	// Name is Kubernetes name of a DNSRecord resource
	Name string `envconfig:"name"`

	// Namespace is the Kubernetes namespace of a DNSRecord resource
	Namespace string `envconfig:"namespace"`
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
var mutex sync.RWMutex

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
