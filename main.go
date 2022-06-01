package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync"

	"github.com/mindtastic/koda/log"

	"github.com/julienschmidt/httprouter"
)

var port = flag.Int("port", 8000, "Port to listen on for API connections")

// AccountKey is a 128 bit value string used to identify users
type AccountKey string

const accountKeyLen = 2*16 + 4

// Record stores multiple user ids
type Record struct {
	userService  string
	wikiService  string
	moodDiary    string
	motivator    string
	notifcations string
}

type Application struct {
	mux        sync.RWMutex
	db         map[AccountKey]Record
	httpServer *http.Server
}

var app *Application

func main() {
	flag.Parse()

	router := httprouter.New()
	// GET /{account_key}			<--- Specific services fetches its user_id for a given account key
	router.GET("/:account_key", ValidateAccountKey(FetchUserId))
	// POST /{account_key}			<--- Creates random user ids for a given account_key
	router.POST("/:account_key", ValidateAccountKey(CreateId))
	// Later:
	// PUT /{account_key}/rotate	<--- Rotates keys for a given ide

	app = &Application{
		db: map[AccountKey]Record{},
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%v", *port),
			Handler: router,
		},
	}

	log.Infof("Listening on %v", app.httpServer.Addr)
	log.Fatal(app.httpServer.ListenAndServe())
}

func FetchUserId(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

}

func CreateId(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	accountKey := AccountKey(ps.ByName("account_key"))
	// record := createRandomRecord()

	app.mux.Lock()
	defer app.mux.Unlock()

	_, exists := app.db[accountKey]
	if exists {
		log.Warnf("Host %v requested id creation for known account_key: %v", r.Host, accountKey)
		http.Error(w, "", http.StatusConflict)
		return
	}

}

func ValidateAccountKey(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		log.Debugf("Validating request '%v'", r.RequestURI)
		accountKey := ps.ByName("account_key")

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

		next(w, r, ps)
	}
}
