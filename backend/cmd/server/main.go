package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
	if err := chromaClient.EnsureMetaCollection(ctx); err != nil {
		log.Fatalf("chroma meta init: %v", err)
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

	http.HandleFunc("/api/health", cors(srv.health))
	http.HandleFunc("/api/ingest", cors(srv.ingest))
	http.HandleFunc("/api/ingest/status", cors(srv.ingestStatus))
	http.HandleFunc("/api/facts", cors(srv.facts))
	http.HandleFunc("/api/query", cors(srv.query))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("OrgLens backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
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

	// Clear previous meta so live status starts fresh
	if err := s.chroma.ResetMeta(ctx); err != nil {
		log.Printf("chroma meta reset: %v", err)
		http.Error(w, "meta reset failed", http.StatusInternalServerError)
		return
	}

	// Extract facts; write per-file meta as each file completes
	onFileDone := func(file string, count int) {
		if err := s.chroma.WriteFileMeta(ctx, file, time.Now().UTC(), count); err != nil {
			log.Printf("write file meta [%s]: %v", file, err)
		}
	}
	facts, err := pipeline.Run(ctx, s.datasetDir, s.nova.ExtractFacts, onFileDone)
	if err != nil {
		log.Printf("pipeline error: %v", err)
		http.Error(w, "pipeline failed", http.StatusInternalServerError)
		return
	}

	// Fresh knowledge collection
	if err := s.chroma.Reset(ctx); err != nil {
		log.Printf("chroma reset: %v", err)
		http.Error(w, "store reset failed", http.StatusInternalServerError)
		return
	}

	// Embed all facts in parallel (bounded to 3 concurrent to stay under Bedrock rate limits)
	log.Printf("Embedding %d facts...", len(facts))
	embeddings := make([][]float64, len(facts))
	sem := make(chan struct{}, 3)
	errc := make(chan error, len(facts))
	for i, f := range facts {
		sem <- struct{}{}
		go func(idx int, text string) {
			defer func() { <-sem }()
			emb, err := s.nova.Embed(ctx, text)
			if err != nil {
				errc <- fmt.Errorf("embed [%d]: %w", idx, err)
				return
			}
			embeddings[idx] = emb
			errc <- nil
		}(i, f.Text)
	}
	for range facts {
		if err := <-errc; err != nil {
			log.Printf("%v", err)
			http.Error(w, "embedding failed", http.StatusInternalServerError)
			return
		}
	}

	if err := s.chroma.Add(ctx, facts, embeddings); err != nil {
		log.Printf("chroma add: %v", err)
		http.Error(w, "store failed", http.StatusInternalServerError)
		return
	}

	log.Printf("Stored %d facts in ChromaDB", len(facts))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"facts_count": len(facts)})
}

func (s *server) ingestStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	files, err := pipeline.WalkDataset(s.datasetDir)
	if err != nil {
		http.Error(w, "walk failed", http.StatusInternalServerError)
		return
	}

	metas, err := s.chroma.GetAllMeta(ctx)
	if err != nil {
		log.Printf("getallmeta: %v", err)
		http.Error(w, "meta read failed", http.StatusInternalServerError)
		return
	}

	metaMap := make(map[string]store.FileMeta, len(metas))
	for _, m := range metas {
		metaMap[m.File] = m
	}

	type fileStatus struct {
		File       string     `json:"file"`
		Ingested   bool       `json:"ingested"`
		IngestedAt *time.Time `json:"ingested_at,omitempty"`
		FactsCount *int       `json:"facts_count,omitempty"`
	}

	statuses := make([]fileStatus, len(files))
	for i, path := range files {
		rel, _ := filepath.Rel(s.datasetDir, path)
		if m, ok := metaMap[rel]; ok {
			t := m.IngestedAt
			n := m.FactsCount
			statuses[i] = fileStatus{File: rel, Ingested: true, IngestedAt: &t, FactsCount: &n}
		} else {
			statuses[i] = fileStatus{File: rel}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statuses)
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

func (s *server) query(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	var req struct {
		Q string `json:"q"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Q == "" {
		http.Error(w, "body must be {\"q\": \"...\"}", http.StatusBadRequest)
		return
	}

	emb, err := s.nova.Embed(ctx, req.Q)
	if err != nil {
		log.Printf("query embed: %v", err)
		http.Error(w, "embed failed", http.StatusInternalServerError)
		return
	}

	facts, err := s.chroma.Query(ctx, emb, 10)
	if err != nil {
		log.Printf("query search: %v", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	answer, err := s.nova.Synthesize(ctx, req.Q, facts)
	if err != nil {
		log.Printf("query synthesize: %v", err)
		http.Error(w, "synthesis failed", http.StatusInternalServerError)
		return
	}

	type source struct {
		Text   string `json:"text"`
		Source string `json:"source"`
	}
	sources := make([]source, len(facts))
	for i, f := range facts {
		sources[i] = source{Text: f.Text, Source: f.Source}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"answer":  answer,
		"sources": sources,
	})
}
