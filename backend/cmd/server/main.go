package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/enas/orglens/internal/nova"
	"github.com/enas/orglens/internal/pipeline"
	"github.com/enas/orglens/internal/store"
)

type server struct {
	nova       *nova.Client
	chroma     *store.Client
	datasetDir string
}

func main() {
	ctx := context.Background()

	novaClient, err := nova.NewClient(ctx)
	if err != nil {
		log.Fatalf("nova init: %v", err)
	}

	chromaClient := store.NewClient()
	if err := chromaClient.EnsureCollection(ctx); err != nil {
		log.Fatalf("chroma init: %v", err)
	}

	datasetDir := os.Getenv("DATASET_DIR")
	if datasetDir == "" {
		datasetDir = "../dataset"
	}

	srv := &server{
		nova:       novaClient,
		chroma:     chromaClient,
		datasetDir: datasetDir,
	}

	http.HandleFunc("/api/health", srv.health)
	http.HandleFunc("/api/ingest", srv.ingest)
	http.HandleFunc("/api/facts", srv.facts)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("OrgLens backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *server) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	// 1. Extract facts from dataset
	facts, err := pipeline.Run(ctx, s.datasetDir, s.nova.ExtractFacts)
	if err != nil {
		log.Printf("pipeline error: %v", err)
		http.Error(w, "pipeline failed", http.StatusInternalServerError)
		return
	}

	// 2. Fresh ingest — reset collection
	if err := s.chroma.Reset(ctx); err != nil {
		log.Printf("chroma reset: %v", err)
		http.Error(w, "store reset failed", http.StatusInternalServerError)
		return
	}

	// 3. Embed each fact
	log.Printf("Embedding %d facts...", len(facts))
	embeddings := make([][]float64, len(facts))
	for i, f := range facts {
		emb, err := s.nova.Embed(ctx, f.Text)
		if err != nil {
			log.Printf("embed [%d]: %v", i, err)
			http.Error(w, "embedding failed", http.StatusInternalServerError)
			return
		}
		embeddings[i] = emb
	}

	// 4. Store in ChromaDB
	if err := s.chroma.Add(ctx, facts, embeddings); err != nil {
		log.Printf("chroma add: %v", err)
		http.Error(w, "store failed", http.StatusInternalServerError)
		return
	}

	log.Printf("Stored %d facts in ChromaDB", len(facts))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"facts_count": len(facts)})
}

func (s *server) facts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	facts, err := s.chroma.GetAll(ctx)
	if err != nil {
		log.Printf("getall: %v", err)
		http.Error(w, "store read failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(facts)
}
