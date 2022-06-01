package main

import (
	"context"
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

func main() {
	flag.Parse()

	// Later:
	// PUT /{account_key}/rotate	<--- Rotates keys for a given ide

	app = &Application{
		db: map[AccountKey]Record{},
		httpServer: &http.Server{
			Addr:    *addr,
			Handler: validateRequest(handleRequest()),
		},
	}

	log.Infof("Listening on %v", app.httpServer.Addr)
	log.Fatal(app.httpServer.ListenAndServe())
}

type ctxKey string

const (
	accountKeyHeader = "X-AccountKey"
	serviceHeader    = "X-ForService"
	userIDHeader     = "X-User-ID"

	accountKeyCtxKey ctxKey = "accountkey"
	serviceCtxKey    ctxKey = "service"
)

func handleRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountKey, ok := r.Context().Value(accountKeyCtxKey).(AccountKey)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		app.mux.Lock()
		record, ok := app.db[accountKey]
		if !ok {
			record = Record{
				serviceKeys: make(map[string]string),
			}
			app.db[accountKey] = record
		}
		app.mux.Unlock()

		service, ok := r.Context().Value(serviceCtxKey).(string)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		serviceUserId, ok := record.serviceKeys[service]
		if !ok {
			id, err := uuid.GenerateUUID()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			app.mux.Lock()
			serviceUserId = id
			record.serviceKeys[service] = serviceUserId
			app.db[accountKey] = record
			app.mux.Unlock()
		}

		w.Header().Set(userIDHeader, fmt.Sprintf("Bearer %s", serviceUserId))
		w.WriteHeader(http.StatusOK)
	}
}

func validateRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountKey := r.Header.Get(accountKeyHeader)
		if accountKey == "" {
			http.Error(w, fmt.Sprintf("missing required header: %v", accountKeyHeader), http.StatusBadRequest)
			return
		}

		_, err := uuid.ParseUUID(accountKey)
		if err != nil {
			http.Error(w, "account key is improperly formatted", http.StatusBadRequest)
			return
		}

		serviceName := r.Header.Get(serviceHeader)
		if serviceName == "" {
			http.Error(w, fmt.Sprintf("missing required header: %v", serviceHeader), http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), accountKeyCtxKey, AccountKey(accountKey))
		ctx = context.WithValue(ctx, serviceCtxKey, serviceName)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
