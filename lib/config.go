package lib

import (
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type Config struct {
	SentryDSN string `yaml:"sentry_dsn,omitempty"`

	Store StoreConfig `yaml:"store"`
	DNS   DNSConfig   `yaml:"dns"`
	Net   NetConfig   `yaml:"net"`
	HTTP  HTTPConfig  `yaml:"http"`

	Provider ProviderConfig `yaml:"provider"`
}

type ProviderConfig struct {
	PWordlistID PWordlistIDConfig `yaml:"wordlist"`
	PRandomID   PRandomIDConfig   `yaml:"random"`
}

type PRandomIDConfig struct {
	Enable bool  `yaml:"enable"`
	IDLen  int16 `yaml:"id_len"`
}

type PWordlistIDConfig struct {
	Enable bool `yaml:"enable"`
}

type DNSConfig struct {
	Address string `yaml:"address"`
	Port    int16  `yaml:"port"`

	MNAME     string   `yaml:"mname"`
	NS        []string `yaml:"ns"`
	DNSSECKey string   `yaml:"dnssec_key,omitempty"`
}

type NetConfig struct {
	Address string `yaml:"address"`
	Port    int16  `yaml:"port"`
}

type HTTPConfig struct {
	Address string `yaml:"address"`
	Port    int16  `yaml:"port"`
}

type StoreConfig struct {
	Domain string        `yaml:"domain"`
	File   string        `yaml:"file"`
	TTL    time.Duration `yaml:"ttl"`
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
