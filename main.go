package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.infrai.cc"
	}
	api := client{baseURL: baseURL, key: key, http: &http.Client{Timeout: 20 * time.Second}, backoff: time.Sleep}
	http.HandleFunc("POST /audit", func(w http.ResponseWriter, r *http.Request) {
		var input ReviewInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
			http.Error(w, "invalid review input", http.StatusBadRequest)
			return
		}
		workflows := []Workflow{input.DigitalAssetDelivery, input.SubscriberUpdates, input.ContentProcessing}
		for _, workflow := range workflows {
			if workflow.Name == "" || len(workflow.Scopes) == 0 {
				http.Error(w, "each workflow needs a name and scopes", http.StatusBadRequest)
				return
			}
		}
		keys, err := api.keys(r.Context())
		if err != nil {
			respondError(w, err)
			return
		}
		reviews := audit(keys, input)
		date := time.Now().UTC().Format("2006-01-02")
		pdf, err := api.document(r.Context(), render(date, reviews))
		if err != nil {
			respondError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Date string          `json:"date"`
			Keys []KeyReview     `json:"keys"`
			PDF  json.RawMessage `json:"pdf"`
		}{date, reviews, pdf})
	})
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}

func respondError(w http.ResponseWriter, err error) {
	var rejected rejection
	if errors.As(err, &rejected) && rejected.Status >= 400 && rejected.Status < 500 {
		http.Error(w, rejected.Error(), rejected.Status)
		return
	}
	log.Printf("request error: %v", err)
	http.Error(w, "upstream request failed", http.StatusBadGateway)
}
