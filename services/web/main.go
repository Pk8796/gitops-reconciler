package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

var requestCount uint64

func main() {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://api:8080"
	}

	client := &http.Client{Timeout: 3 * time.Second}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/metrics", handleMetrics)
	mux.HandleFunc("/", handleIndex(client, apiURL))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("web listening on :%s, calling api at %s", port, apiURL)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP web_requests_total Total requests handled\n")
	fmt.Fprintf(w, "# TYPE web_requests_total counter\n")
	fmt.Fprintf(w, "web_requests_total %d\n", atomic.LoadUint64(&requestCount))
}

// handleIndex is a thin proxy - web's whole job is to prove the two services
// can find each other over the cluster network, nothing fancier than that.
func handleIndex(client *http.Client, apiURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&requestCount, 1)

		target := apiURL + "/shorten"
		if q := r.URL.Query().Get("url"); q != "" {
			target += "?url=" + q
		}

		resp, err := client.Get(target)
		if err != nil {
			http.Error(w, "api unreachable: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
	}
}
