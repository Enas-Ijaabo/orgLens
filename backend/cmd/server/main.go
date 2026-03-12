package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/enas/orglens/internal/nova"
)

func main() {
	ctx := context.Background()

	novaClient, err := nova.NewClient(ctx)
	if err != nil {
		log.Fatalf("nova init: %v", err)
	}

	facts, err := novaClient.ExtractFacts(ctx, "AuthService handles authentication using JWT tokens.", "test")
	if err != nil {
		log.Fatalf("extract facts: %v", err)
	}
	for _, f := range facts {
		log.Printf("Fact: {Subject:%s Relation:%s Object:%s}", f.Subject, f.Relation, f.Object)
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
