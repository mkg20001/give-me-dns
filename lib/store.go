package lib

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/mkg20001/give-me-dns/lib/idprov"
	bolt "go.etcd.io/bbolt"
	"net"
	"sync"
	"time"
)

type Entry struct {
	Expires time.Time `json:"expires"`
	Value   net.IP    `json:"value"`
}

type Store struct {
	db         *bolt.DB
	file       string
	serial     int64
	openLock   sync.Mutex
	openCancel context.CancelFunc
	Config     *StoreConfig
	providers  []idprov.IDProv
}

func ProvideStore(config *StoreConfig, providers []idprov.IDProv) (error, func() error, *Store) {
	store := &Store{
		Config:    config,
		file:      config.File,
		providers: providers,
	}
	err := store.Open()
	if err != nil {
		return err, nil, nil
	}

	return nil, func() error {
		return store.Close()
	}, store
}

func (s *Store) Domain() string {
	return s.Config.Domain
}

func (s *Store) TTL() time.Duration {
	return s.Config.TTL
}

func (s *Store) AssertDB() error {
	if s.db == nil {
		return bolt.ErrDatabaseNotOpen
	}

	return nil
}

func (s *Store) Open() error {
	if s.db != nil { // Idempotent
		return nil
	}

	s.openLock.Lock()
	defer s.openLock.Unlock()

	db, err := bolt.Open(s.file, 0600, nil)
	if err != nil {
		return err
	}
	s.db = db

	err = s.db.Update(func(tx *bolt.Tx) error {
		// key: dns entry id - value: ip
		bDNS, err := tx.CreateBucketIfNotExists([]byte("dns"))
		if err != nil {
			return err
		}

		// key: ip - value: dns entry id
		bIP, err := tx.CreateBucketIfNotExists([]byte("dns4ip"))
		if err != nil {
			return err
		}

		var entryParsed Entry
		now := time.Now()

		c := bDNS.Cursor()
		for id, entry := c.First(); id != nil; id, entry = c.Next() {
			err := json.Unmarshal(entry, &entryParsed)
			if err != nil {
				return err
			}

			if entryParsed.Expires.Before(now) {
				err := bIP.Delete(entryParsed.Value)
				if err != nil {
					return err
				}

				err = bDNS.Delete(id)
				if err != nil {
					return err
				}
			} else {
				if s.serial < entryParsed.Expires.Unix() {
					s.serial = entryParsed.Expires.Unix()
				}
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.openCancel = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(60 * 3600 * time.Second):
				err := s.db.Update(func(tx *bolt.Tx) error {
					bDNS := tx.Bucket([]byte("dns"))
					bIP := tx.Bucket([]byte("dns4ip"))

					now := time.Now()

					c := bDNS.Cursor()
					var entryParsed Entry
					for id, entry := c.First(); id != nil; id, entry = c.Next() {
						err := json.Unmarshal(entry, &entryParsed)
						if err != nil {
							return err
						}

						if entryParsed.Expires.Before(now) {
							err := bIP.Delete(entryParsed.Value)
							if err != nil {
								return err
							}

							err = bDNS.Delete(id)
							if err != nil {
								return err
							}
						}
					}

					return nil
				})

				if err != nil {
					sentry.CaptureException(err)
					return
				}
			}
		}
	}()

	return nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}

	s.openLock.Lock()
	defer s.openLock.Unlock()

	s.openCancel()
	s.openCancel = nil

	err := s.db.Close()
	if err != nil {
		return err
	}

	s.db = nil

	return nil
}

func (s *Store) AddEntry(ipaddr net.IP) (Entry, string, error) {
	var entry Entry
	var id string

	err := s.AssertDB()
	if err != nil {
		return entry, id, err
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		bDNS := tx.Bucket([]byte("dns"))
		bIP := tx.Bucket([]byte("dns4ip"))

		idByte := bIP.Get(ipaddr)
		provId := -1
		maxTries := 50
		if idByte == nil {
		genID:
			provId = (provId + 1) % len(s.providers)
			maxTries = maxTries - 1
			if maxTries == 0 {
				return errors.New("could not find any free id")
			}
			id, err := s.providers[provId].GetID()
			if err != nil {
				return err
			}
			idByte = []byte(id)
			existingEntry := bDNS.Get(idByte)
			if existingEntry != nil {
				goto genID
			}

			err = bIP.Put(ipaddr, idByte)
			if err != nil {
				return err
			}
		}

		entry = Entry{
			Expires: time.Now().Add(s.Config.TTL),
			Value:   ipaddr,
		}
		s.serial = entry.Expires.Unix()
		marshal, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		err = bDNS.Put(idByte, marshal)
		if err != nil {
			return err
		}

		id = string(idByte) + "." + s.Config.Domain

		return nil
	})

	return entry, id, err
}

func (s *Store) ResolveEntry(id string) (net.IP, error) {
	err := s.AssertDB()
	if err != nil {
		return nil, err
	}

	var ip net.IP

	err = s.db.View(func(tx *bolt.Tx) error {
		bDNS := tx.Bucket([]byte("dns"))
		entry := bDNS.Get([]byte(id))
		if entry == nil {
			return nil
		}
		var entryParsed Entry
		err := json.Unmarshal(entry, &entryParsed)
		if err != nil {
			return err
		}

		ip = entryParsed.Value
		return nil
	})

	return ip, err
}

func (s *Store) ResolveIP(ip net.IP) (Entry, string, error) {
	var entryParsed Entry
	var idStr string

	err := s.AssertDB()
	if err != nil {
		return entryParsed, idStr, err
	}

	err = s.db.View(func(tx *bolt.Tx) error {
		bDNS := tx.Bucket([]byte("dns"))
		bIP := tx.Bucket([]byte("dns4ip"))

		id := bIP.Get(ip)
		if id == nil {
			return nil
		}

		idStr = string(id) + "." + s.Config.Domain
		entry := bDNS.Get(id)
		if entry == nil {
			return nil
		}

		err := json.Unmarshal(entry, &entryParsed)
		if err != nil {
			return err
		}

		return nil
	})

	return entryParsed, idStr, err
}

func (s *Store) GetSerial() uint32 {
	return uint32(s.serial)
}
