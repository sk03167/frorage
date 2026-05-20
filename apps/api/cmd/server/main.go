package main

import (
	"log"
	"net/http"
	"time"

	"frorage/apps/api/internal/config"
	"frorage/apps/api/internal/httpapi"
	"frorage/apps/api/internal/objectstore"
	"frorage/apps/api/internal/store"
)

func main() {
	cfg := config.FromEnv()
	repo := store.NewMemoryRepository()
	objects := objectstore.NewS3Presigner(cfg.S3)
	server := httpapi.NewServer(cfg, repo, objects)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("api listening on %s", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
