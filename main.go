package main

import (
	"log"
	"net/http"
	"os"

	handler "autorun-go/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.Handler)
	mux.HandleFunc("/api", handler.Handler)
	mux.HandleFunc("/api/", handler.Handler)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("autorun-go listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

