package localfile

import (
	"fmt"
	"github.com/mindtastic/koda"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestLocalFileStore_InitializePersistence(t *testing.T) {
	testCases := []struct {
		name         string
		persistence  bool
		noPermission bool
		err          string
	}{
		{name: "no persistence", persistence: false, noPermission: false, err: ""},
		{name: "test directory", persistence: true, noPermission: false, err: ""},
		{name: "no permission", persistence: true, noPermission: true, err: "error opening file: open %s: permission denied"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var dbPath string
			if tc.persistence {
				f, err := os.CreateTemp("", "koda-testing-*")
				if err != nil {
					t.Fatalf("error creating temporary file: %v", err)
				}
				dbPath = f.Name()

				if tc.noPermission {
					os.Chmod(f.Name(), 0)
				}

				f.Close()
			}

			lfs := New()
			err := lfs.InitializePersistence(dbPath)
			if tc.err != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, fmt.Sprintf(tc.err, dbPath))
				return
			} else {
				assert.NoError(t, err)
				assert.Equal(t, dbPath, lfs.dbPath)
			}
		})
	}
}

func TestFlush(t *testing.T) {
	testCases := []struct {
		name         string
		record       koda.Record
		expectedSize int64
		noPermission bool
		err          string
	}{
		{name: "no data", record: koda.Record{AccountKey: ""}, expectedSize: 58, err: ""},
		{name: "success", record: koda.Record{AccountKey: "testing"}, expectedSize: 72, err: ""},
		{name: "no persistence", record: koda.Record{AccountKey: "testing"}, expectedSize: 72, err: ""},
		{name: "no permission", record: koda.Record{AccountKey: "testing"}, noPermission: true, err: "error writing data file %s: open %s: permission denied"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var dbPath string
			f, err := os.CreateTemp("", "koda-testing-*")
			if err != nil {
				t.Fatalf("error creating temporary file: %v", err)
			}
			dbPath = f.Name()
			f.Close()

			lfs := New()
			err = lfs.InitializePersistence(dbPath)
			assert.NoError(t, err)
			err = lfs.Set(tc.record.AccountKey, tc.record)
			assert.NoError(t, err)

			if tc.noPermission {
				os.Chmod(f.Name(), 0)
			}

			err = lfs.flush()
			if tc.err != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, fmt.Sprintf(tc.err, dbPath, dbPath))
				return
			} else {
				assert.NoError(t, err)
				assert.Equal(t, dbPath, lfs.dbPath)
			}

			info, err := os.Stat(dbPath)
			if err != nil {
				t.Fatalf("error getting file info for file %q: %v", dbPath, err)
			}
			assert.Equal(t, tc.expectedSize, info.Size())
		})
	}
}
