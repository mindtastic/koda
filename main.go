package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"

	"github.com/hashicorp/go-uuid"
	"github.com/mindtastic/koda/log"
)

var addr = flag.String("addr", ":8000", "Address to listen on for API connections")

// AccountKey is a 128 bit value string used to identify users
type AccountKey string

// Record stores multiple user ids
type Record struct {
	serviceKeys map[string]string
}

type Application struct {
	mux        sync.RWMutex
	db         map[AccountKey]Record
	httpServer *http.Server
}

var app *Application

func (a *Application) recordForKey(key AccountKey) (Record, bool) {
	a.mux.RLock()
	defer a.mux.RUnlock()
	record, ok := app.db[key]
	return record, ok
}

func (a *Application) getUserIdsFor(key AccountKey) Record {
	record, ok := a.recordForKey(key)
	if !ok {
		a.mux.Lock()
		record = Record{
			serviceKeys: make(map[string]string),
		}
		app.db[key] = record
		a.mux.Unlock()
	}
	return record
}

func main() {
	flag.Parse()

	// Later:
	// PUT /{account_key}/rotate	<--- Rotates keys for a given ide

	app = &Application{
		db: map[AccountKey]Record{},
		httpServer: &http.Server{
			Addr:    *addr,
			Handler: handleRequest(),
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
		URL                 string   `json:"url"`
	} `json:"match_context"`
}

func (p OathkeeperPayload) ServiceName() string {
	log.Warnf("Service extracted from payload is hardcoded to user-service")
	return "user-service"
}

func handleRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "invalid method", http.StatusBadRequest)
			return
		}

		var requestPayload OathkeeperPayload
		d := json.NewDecoder(r.Body)
		if err := d.Decode(&requestPayload); err != nil {
			http.Error(w, fmt.Sprintf("malformed request: %v", err), http.StatusBadRequest)
			return
		}

		if _, err := uuid.ParseUUID(requestPayload.Subject); err != nil {
			http.Error(w, "subject must be a valid account key", http.StatusBadRequest)
			return
		}
		accountKey := AccountKey(requestPayload.Subject)

		record := app.getUserIdsFor(accountKey)
		service := requestPayload.ServiceName()

		app.mux.RLock()
		serviceUserId, ok := record.serviceKeys[service]
		app.mux.RUnlock()
		if !ok {
			id, err := uuid.GenerateUUID()
			if err != nil {
				log.Errorf("error generating new id: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			app.mux.Lock()
			serviceUserId = id
			record.serviceKeys[service] = serviceUserId
			app.db[accountKey] = record
			app.mux.Unlock()
		}

		response := requestPayload
		response.Extra[userIdExtraKey] = serviceUserId

		e := json.NewEncoder(w)
		if err := e.Encode(response); err != nil {
			log.Errorf("error json encoding response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
