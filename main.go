package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"sync"

	"github.com/mindtastic/koda/log"
)

var port = flag.Int("port", 8000, "Port to listen on for API connections")

// AccountKey is a 128 bit value string used to identify users
type AccountKey string

const accountKeyLen = 2*16 + 4

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
			Addr:    fmt.Sprintf(":%v", *port),
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

	accountKeyCtxKey ctxKey = "accountkey"
	serviceCtxKey    ctxKey = "service"
)

func handleRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountKey := r.Context().Value(accountKeyCtxKey).(AccountKey)

		record, found := app.db[accountKey]
		if !found {
			record = Record{
				serviceKeys: make(map[string]string),
			}
			app.db[accountKey] = record
		}

		service := r.Context().Value(serviceCtxKey).(string)
		serviceUserId, found := record.serviceKeys[service]
		if !found {
			serviceUserId = randomUserId()
			record.serviceKeys[service] = serviceUserId
			app.db[accountKey] = record
		}

		w.Header().Set("Authentication", fmt.Sprintf("Bearer %s", serviceUserId))
		w.WriteHeader(http.StatusOK)
	}
}

func randomUserId() string {
	panic("Not implemented")
	return ""
}

func validateRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountKey := r.Header.Get(accountKeyHeader)
		if accountKey == "" {
			http.Error(w, fmt.Sprintf("missing required header: %v", accountKeyHeader), http.StatusBadRequest)
			return
		}

		if len(accountKey) != accountKeyLen {
			log.Debugf("account key string '%v' rejected for invalid length", accountKey)
			http.Error(w, "account key string is wrong length", http.StatusBadRequest)
			return
		}

		if accountKey[8] != '-' ||
			accountKey[13] != '-' ||
			accountKey[18] != '-' ||
			accountKey[23] != '-' {
			log.Debugf("account key string '%v' rejected for invalid format", accountKey)
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
