package main

import (
	"context"
	"flag"
	"github.com/mindtastic/koda"
	"github.com/mindtastic/koda/store/localfile"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mindtastic/koda/log"
)

var addr = flag.String("addr", ":8000", "Address to listen on for API connections")
var dbpath = flag.String("db", "/data/db/koda.db", "File to store database")

type application struct {
	mu         sync.RWMutex
	store      koda.Store
	httpServer *http.Server
}

func main() {
	flag.Parse()

	lfs := localfile.New()
	if err := lfs.InitializePersistence(*dbpath); err != nil {
		log.Fatalf("error initializing database: %v", err)
	}

	app := &application{
		httpServer: &http.Server{
			Addr: *addr,
		},
		store: lfs,
	}

	app.httpServer.Handler = app.initializeMux()
	shutdown := make(chan os.Signal)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("error listening on address %q: %v", app.httpServer.Addr, err)
		}
	}()
	log.Infof("listening on address %q", app.httpServer.Addr)
	<-shutdown

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.httpServer.Shutdown(ctx); err != nil {
		log.Errorf("error shutting down server: %v", err)
	}
	if err := lfs.Shutdown(); err != nil {
		log.Errorf("error shutting down database: %v", err)
	}
}
