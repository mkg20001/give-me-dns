package lib

import (
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type Config struct {
	Domain string        `yaml:"domain"`
	TTL    time.Duration `yaml:"ttl"`
	IDLen  int16         `yaml:"idlen"`

	DNSAddress string `yaml:"dns_address"`
	DNSPort    int16  `yaml:"dns_port"`
	DNSMNAME   string `yaml:"dns_mname"`
	DNSNS      string `yaml:"dns_ns"`

	NetAddress string `yaml:"net_address"`
	NetPort    int16  `yaml:"net_port"`

	StoreFile string `yaml:"store_file"`

	SentryDSN string `yaml:"sentry_dsn,omitempty"`
}

func ReadConfig(path string) (*Config, error) {
	yfile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(yfile, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
