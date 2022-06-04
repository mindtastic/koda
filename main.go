package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/hashicorp/go-uuid"
	"github.com/mindtastic/koda/log"
	"github.com/mindtastic/koda/logstore"
)

var addr = flag.String("addr", ":8000", "Address to listen on for API connections")

// AccountKey is a 128 bit value string used to identify users
type AccountKey = string

// Record stores multiple user ids
type Record struct {
	ServiceKeys map[string]string
}

func NewRecord() *Record {
	return &Record{
		ServiceKeys: make(map[string]string),
	}
}

type Application struct {
	db         *logstore.Store
	httpServer *http.Server
}

var app *Application

func (a *Application) storeRecord(k AccountKey, r *Record) error {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)

	if err := enc.Encode(*r); err != nil {
		return fmt.Errorf("error serializing record to binary: %v", err)
	}

	return a.db.Set(k, buffer.Bytes())
}

func (a *Application) fetchRecordByKey(key AccountKey) (*Record, error) {
	rawRecord, err := a.db.Get(key)
	if logstore.IsNotFoundError(err) {
		// The account key does not exist so far in the database. Create it one-the-fly.
		return NewRecord(), nil
	} else if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(rawRecord)
	dec := gob.NewDecoder(buf)

	var r Record
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("error decoding fetched binary data for key %v: %v", key, err)
	}

	return &r, nil
}

func main() {
	flag.Parse()

	workdir, err := os.Getwd()
	if err != nil {
		log.Fatalf("error accessing working directory for database: %v", err)
	}

	db, err := logstore.NewStore(workdir)
	if err != nil {
		log.Fatalf("error creating/accessing db: %v", err)
	}

	mux := http.ServeMux{}
	mux.Handle("/", handleRequest())
	mux.Handle("/health", handleHealthcheck())
	// Later:
	// PUT /{account_key}/rotate	<--- Rotates keys for a given ide

	app = &Application{
		db: db,
		httpServer: &http.Server{
			Addr:    *addr,
			Handler: &mux,
		},
	}

	log.Infof("Listening on %v", app.httpServer.Addr)
	log.Fatal(app.httpServer.ListenAndServe())
}

const (
	userIdExtraKey = "userID"
)

type OathkeeperPayload struct {
	Subject      string                 `json:"subject"`
	Extra        map[string]interface{} `json:"extra"`
	Header       http.Header            `json:"header"`
	MatchContext struct {
		RegexpCaptureGroups []string `json:"regexp_capture_groups"`
		URL                 url.URL  `json:"url"`
	} `json:"match_context"`
}

func (p OathkeeperPayload) ServiceName() string {
	log.Warnf("Service extracted from payload is hardcoded to user-service")
	return "user-service"
}

func handleHealthcheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func handleRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			log.Errorf("received non-POST request")
			http.Error(w, "invalid method", http.StatusBadRequest)
			return
		}

		var requestPayload OathkeeperPayload
		d := json.NewDecoder(r.Body)
		if err := d.Decode(&requestPayload); err != nil {
			log.Errorf("error decoding JSON body: %v", err)
			http.Error(w, fmt.Sprintf("malformed request: %v", err), http.StatusBadRequest)
			return
		}

		if _, err := uuid.ParseUUID(requestPayload.Subject); err != nil {
			log.Errorf("error parsing accountKey: %v", err)
			http.Error(w, "subject must be a valid account key", http.StatusBadRequest)
			return
		}
		accountKey := AccountKey(requestPayload.Subject)

		record, err := app.fetchRecordByKey(accountKey)
		if err != nil {
			log.Errorf("database error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		service := requestPayload.ServiceName()
		serviceUserId, ok := record.ServiceKeys[service]
		if !ok {
			id, err := uuid.GenerateUUID()
			if err != nil {
				log.Errorf("error generating new service ID: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			serviceUserId = id
			record.ServiceKeys[service] = serviceUserId
			err = app.storeRecord(accountKey, record)
			if err != nil {
				log.Errorf("database error: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		response := requestPayload
		response.Extra[userIdExtraKey] = serviceUserId

		e := json.NewEncoder(w)
		if err := e.Encode(response); err != nil {
			log.Errorf("error encoding JSON response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
