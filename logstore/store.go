package logstore

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/mindtastic/koda/log"
)

const (
	logFileName          = "logstore.db"
	defaultMaxRecordSize = 1 << 20 // 1 Megabyte
)

// Store represents a persistent, append only logbased key-value store
type Store struct {
	// Path of the underlying logfile
	storagePath string
	// Maximum allowed size ofr a single record
	maxRecordSize int
	// Set the sync flag to actually write to disk (using sync systemcall) after each database write access.
	// Synchronous mode might cause dramatic performance decrease.
	sync bool

	mu sync.Mutex
}

func NewStore(storeDir string) (*Store, error) {
	p := path.Join(storeDir, logFileName)

	if _, err := os.OpenFile(p, os.O_CREATE, 0600); err != nil {
		return nil, err
	}

	return &Store{
		storagePath:   p,
		maxRecordSize: defaultMaxRecordSize,
	}, nil
}

func (s *Store) Get(key string) ([]byte, error) {
	f, err := os.Open(s.storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db file %v: %v", s.storagePath, err)
	}
	defer f.Close()

	scanner, err := NewScanner(f, s.maxRecordSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create scanner for %v: %v", s.storagePath, err)
	}

	var r *Record
	for scanner.Scan() {
		read := scanner.Record()
		if read.key == key {
			r = read
		}
	}

	if scanner.Err() != nil {
		log.Errorf("error encountered on reading db: %v", scanner.Err())
		return nil, scanner.Err()
	}

	if r == nil || r.IsTombstone() {
		return nil, NewNotFoundError(key)
	}

	return r.Value(), nil
}

func (s *Store) Set(key string, val []byte) error {
	r := NewRecord(key, val)
	return s.append(r)
}

func (s *Store) Delete(key string) error {
	r := NewTombstone(key)
	return s.append(r)
}

// Append a record to the log. Function is used internally to store new and
// delete records.
func (s *Store) append(r *Record) error {
	if r.Size() > s.maxRecordSize {
		return NewBadRequestError(fmt.Sprintf("value to big. max. allowed size is: %v (got: %v)", s.maxRecordSize, r.Size()))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.storagePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open db file %v for writing: %v", s.storagePath, err)
	}
	defer f.Close()

	bytesWritten, err := r.Write(f)
	if err != nil {
		return fmt.Errorf("failed to write record to file %v: %v", s.storagePath, err)
	}
	log.Infof("Wrotes record of %d bytes to database", bytesWritten)

	if s.sync {
		return f.Sync()
	}

	return f.Close()
}
