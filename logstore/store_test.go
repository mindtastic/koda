package logstore

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func initWorkdir() (string, error) {
	return os.MkdirTemp("", "")
}

func teardown(path string) {
	os.RemoveAll(path)
}

func TestStore(t *testing.T) {
	dir, err := initWorkdir()
	if err != nil {
		t.Fatalf("failed to bootstrap test db dir: %v", err)
	}
	defer teardown(dir)

	testKey := "my-precious-testkey"
	testValue := "this-will-soon-become-binary"

	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	err = store.Set(testKey, []byte(testValue))
	if err != nil {
		t.Fatalf("error storing to db: %v", err)
	}

	data, err := store.Get(testKey)
	if err != nil {
		t.Fatalf("error fetching from db: %v", err)
	}

	if !bytes.Equal(data, []byte(testValue)) {
		t.Errorf("got data from db, but it didn't match the expected value")
	}

	err = store.Delete(testKey)
	if err != nil {
		t.Fatalf("Error deleting from database")
	}

	var notFoundErr *NotFoundError
	data, err = store.Get(testKey)
	if !errors.As(err, &notFoundErr) {
		if err != nil {
			t.Fatalf("internal error on fetching from db: %v", err)
		}

		t.Errorf("expected a NotFoundError for key %v after deleting from store (got: %v)", testKey, data)
	}
}
