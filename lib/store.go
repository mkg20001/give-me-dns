package lib

import (
	"encoding/json"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
	"net"
	"sync"
	"time"
)

type storeEntry struct {
	Expires time.Time `json:"expires"`
	Value   net.IP    `json:"value"`
}

type Store struct {
	db       *bolt.DB
	file     string
	serial   int64
	openLock sync.Mutex
	Config   *Config
}

func ProvideStore(config *Config) (error, func() error, *Store) {
	store := &Store{
		Config: config,
		file:   config.StoreFile,
	}
	err := store.Open()
	if err != nil {
		return err, nil, nil
	}

	return nil, func() error {
		return store.Close()
	}, store
}

func genID(l int16) (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return id.String()[0:l], nil
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

		var entryParsed storeEntry
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

	return nil
}

func (s *Store) Close() error {
	err := s.db.Close()
	if err != nil {
		return err
	}

	s.db = nil

	return nil
}

func (s *Store) AddEntry(ipaddr []byte) (string, error) {
	err := s.AssertDB()
	if err != nil {
		return "", err
	}

	var id string

	err = s.db.Update(func(tx *bolt.Tx) error {
		bDNS := tx.Bucket([]byte("dns"))
		bIP := tx.Bucket([]byte("dns4ip"))

		idByte := bIP.Get(ipaddr)
		if idByte == nil {
		genID:
			id, err := genID(s.Config.IDLen)
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

		entry := storeEntry{
			Expires: time.Now(),
			Value:   ipaddr,
		}
		marshal, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		err = bDNS.Put(idByte, marshal)
		if err != nil {
			return err
		}

		id = string(idByte)

		return nil
	})

	return id, err
}

func (s *Store) ResolveEntry(id string) ([]byte, error) {
	err := s.AssertDB()
	if err != nil {
		return nil, err
	}

	var ip []byte

	err = s.db.View(func(tx *bolt.Tx) error {
		bDNS := tx.Bucket([]byte("dns"))
		entry := bDNS.Get([]byte(id))
		if entry == nil {
			return nil
		}
		var entryParsed storeEntry
		err := json.Unmarshal(entry, &entryParsed)
		if err != nil {
			return err
		}

		ip = entryParsed.Value
		return nil
	})

	return ip, err
}

func (s *Store) GetSerial() uint32 {
	return uint32(s.serial)
}
