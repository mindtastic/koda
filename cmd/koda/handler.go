package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/go-uuid"
	"github.com/mindtastic/koda"
	"github.com/mindtastic/koda/log"
	"net/http"
	"net/url"
)

func (a *application) initializeMux() *http.ServeMux {
	mux := new(http.ServeMux)
	mux.Handle("/", a.handleRequest())
	mux.Handle("/health", a.handleHealthcheck())
	//TODO:
	// PUT /{account_key}/rotate	<--- Rotates keys for a given ide

	return mux
}

func (a *application) handleHealthcheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func (a *application) handleRequest() http.HandlerFunc {
	const userIdExtraKey = "userID"

	type payload struct {
		Subject      string                 `json:"subject"`
		Extra        map[string]interface{} `json:"extra"`
		Header       http.Header            `json:"header"`
		MatchContext struct {
			RegexpCaptureGroups []string `json:"regexp_capture_groups"`
			URL                 url.URL  `json:"url"`
		} `json:"match_context"`
	}

	var serviceName = func(p payload) string {
		log.Warnf("Service extracted from payload is hardcoded to user-service")
		return "user-service"
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			log.Errorf("received non-POST request")
			http.Error(w, "invalid method", http.StatusBadRequest)
			return
		}

		var requestPayload payload
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
		accountKey := koda.AccountKey(requestPayload.Subject)

		record, err := a.store.Get(accountKey)
		if err != nil && !errors.Is(err, koda.ErrNotFound) {
			log.Errorf("error getting record for AccountKey %q from store: %v", accountKey, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		serviceName := serviceName(requestPayload)
		serviceUserId, ok := record.ServiceKeys[serviceName]
		if !ok {
			id, err := uuid.GenerateUUID()
			if err != nil {
				log.Errorf("error generating new ServiceKey: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			serviceUserId = koda.ServiceKey(id)
			record.ServiceKeys[serviceName] = serviceUserId
			if err := a.store.Set(accountKey, record); err != nil {
				log.Errorf("error saving record for AccountKey %q: %v", accountKey, err)
				w.WriteHeader(http.StatusInternalServerError)
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
