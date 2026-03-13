package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/enas/orglens/internal/nova"
	"github.com/enas/orglens/internal/pipeline"
)

func main() {
	ctx := context.Background()

	novaClient, err := nova.NewClient(ctx)
	if err != nil {
		log.Fatalf("nova init: %v", err)
	}

	datasetDir := os.Getenv("DATASET_DIR")
	if datasetDir == "" {
		datasetDir = "../dataset"
	}

	allFacts, err := pipeline.Run(ctx, datasetDir, novaClient.ExtractFacts)
	if err != nil {
		log.Fatalf("pipeline: %v", err)
	}

	log.Printf("Extracted %d facts total", len(allFacts))
	for _, f := range allFacts {
		log.Printf("  [%s] %s  [%s]", f.Type, f.Text, f.Source)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Printf("OrgLens backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
