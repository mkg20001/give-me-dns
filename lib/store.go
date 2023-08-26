package lib

import (
	"github.com/google/uuid"
	"sync"
	"time"
)

type storeEntry struct {
	expires time.Time
	value   string
}

type Store struct {
	ips    map[string]*storeEntry
	mu     sync.Mutex
	Config *Config
}

func ProvideStore(config *Config) *Store {
	return &Store{
		ips:    make(map[string]*storeEntry),
		Config: config,
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
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Check if we already have ip
	for id, entry := range s.ips {
		if entry.value == ipaddr {
			entry.expires = time.Now().Add(s.Config.TTL)
			return id, nil
		}

		// Since we run through the entire array at times, do cleanup here
		// This is sloppy, but enough for this service
		if entry.expires.Before(now) {
			delete(s.ips, id)
		}
	}

	// Generate new entry
genID:
	id, err := genID(s.Config.IDLen)

	if err != nil {
		return "", err
	}

	if s.ips[id] != nil {
		goto genID
	}

	s.ips[id] = &storeEntry{
		expires: time.Now().Add(s.Config.TTL),
		value:   ipaddr,
	}
	return id, nil
}

func (s *Store) ResolveEntry(id string) (string, error) {
	ip := ""
	if s.ips[id] != nil {
		ip = s.ips[id].value
	}

	return ip, nil
}
