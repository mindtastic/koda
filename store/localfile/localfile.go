package localfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mindtastic/koda"
	"io"
	"os"
	"path"
	"sync"
	"time"
)

// Ensure that LocalFileStore implements the koda.Store interface
var _ koda.Store = (*LocalFileStore)(nil)

const defaultFlushInterval = 10 * time.Second

var ErrStoreClosed = errors.New("store is closed")

// LocalFileStore is an in memory koda.Store that persists records on disk at regular intervals.
// It is safe for concurrent access, however it should not be used for production workloads as data is not stored encrypted.
type LocalFileStore struct {
	mu            sync.RWMutex
	store         map[koda.AccountKey]koda.Record
	flushInterval time.Duration
	stopped       bool
	shutdown      sync.Once
	dbPath        string // Only set if persistence is enabled
}

// New creates a new LocalFileStore.
// After creating a new LocalFileStore lfs, InitializePersistence should be called to load any existing data or create a new
// store on disk. Not doing so will cause lfs to keep data only in memory and not persist it to disk.
func New() *LocalFileStore {
	lfs := LocalFileStore{
		store:         make(map[koda.AccountKey]koda.Record),
		flushInterval: defaultFlushInterval,
	}
	return &lfs
}

// InitializePersistence initilaizes the persistence layer of LocalFileStore.
// dbpath denotes the path to a data file which will be loaded.
// If it does not exist, it will be created.
// If dbpath is empty, LocalFileStore will not be initialized with persistence and all data is stored in memory only.
func (l *LocalFileStore) InitializePersistence(dbpath string) error {
	if dbpath == "" {
		return nil
	}
	dbFile, err := os.Open(dbpath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("error opening file: %v", err)
		}
		// Ensure path
		p := path.Dir(dbpath)
		if err := os.MkdirAll(p, 0666); err != nil {
			return fmt.Errorf("error creating path %s: %v", p, err)
		}
		f, err := os.Create(dbpath)
		if err != nil {
			return fmt.Errorf("error creating file %s: %v", dbpath, err)
		}
		dbFile = f
	}
	defer dbFile.Close()
	d := json.NewDecoder(dbFile)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dbPath = dbpath
	if err := d.Decode(&l.store); err != nil && err != io.EOF {
		return fmt.Errorf("error decoding existing database file %s: %v", dbpath, err)
	}
	go l.flushAtInterval(l.flushInterval)
	return nil
}

func (l *LocalFileStore) flushAtInterval(i time.Duration) {
	for {
		<-time.Tick(i)
		l.flush()
	}
}

// flush flushes the current state to disk. This is currently done in a syncronous way, meaning the store is blocking any
// writes during the flushing period.
func (l *LocalFileStore) flush() error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.dbPath == "" { // Persistence not enabled.
		return nil
	}
	dd, err := json.Marshal(l.store)
	if err != nil {
		return fmt.Errorf("error encoding data in store: %v", err)
	}
	if err := os.WriteFile(l.dbPath, dd, 0666); err != nil {
		return fmt.Errorf("error writing data file %s: %v", l.dbPath, err)
	}
	return nil
}

// Shutdown gracefully stops the LocalFileStore, ensuring that data is persisted to disk one last time.
// Shutdown works similarly to http.Server.Shutdown() in that it stops any new incoming writes, and then waits indefinitely
// for the data to disk. Shutdown returns any error that occurs during writing data to disk.
// After Shutdown is called, Set and Get immediately return ErrStoreClosed.
// A Closed Store cannot be reused.
func (l *LocalFileStore) Shutdown() error {
	var err error
	l.shutdown.Do(func() {
		l.mu.Lock()
		l.stopped = true
		l.mu.Unlock() // Unlocking immediately to unblock any incoming Set and Get calls.
		err = l.flush()
	})
	return err
}

// Set stores a new record in memory. It will not automatically be flushed to disk.
func (l *LocalFileStore) Set(key koda.AccountKey, record koda.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped {
		return ErrStoreClosed
	}
	l.store[key] = record
	return nil
}

// Get retrieves an existing record. It returns koda.ErrNotFound if the record does not exist.
func (l *LocalFileStore) Get(key koda.AccountKey) (koda.Record, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.stopped {
		return koda.Record{}, ErrStoreClosed
	}
	r, ok := l.store[key]
	if !ok {
		return koda.Record{}, fmt.Errorf("could not get key %s: %w", key, koda.ErrNotFound)
	}
	return r, nil
}
