package lib

import (
	"github.com/google/uuid"
)

type Store struct {
	ips    map[string]string
	config *Config
}

func ProvideStore(config *Config) *Store {
	return &Store{
		ips:    make(map[string]string),
		config: config,
	}
}

func genID(l int16) (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return id.String()[0:l], nil
}

func (s *Store) AddEntry(ipaddr string) (string, error) {
	id, err := genID(s.config.IDLen)

	if err != nil {
		return "", err
	}

	if s.ips[id] != "" {
		return s.AddEntry(ipaddr)
	} else {
		s.ips[id] = ipaddr
		return id, nil
	}
}

func (s *Store) ResolveEntry(id string) (string, error) {
	return s.ips[id], nil
}
