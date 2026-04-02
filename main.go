package main

import (
	"log"
	"net/http"

	"code.byted.org/fintech_cf/common-notify/internal/api"
	"code.byted.org/fintech_cf/common-notify/internal/store"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
)

func main() {
	s, err := store.NewSQLiteStore("./notifications.db")
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	w := worker.NewWorker(s, 3, 1000)
	w.Start()

	h := api.NewHandler(s, w)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	log.Println("server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
