package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	requestCount uint64
	errorCount   uint64
)

type healthResp struct {
	Status string `json:"status"`
}

type shortenResp struct {
	ShortCode   string `json:"short_code"`
	OriginalURL string `json:"original_url"`
	Hits        uint64 `json:"hits"`
}

func main() {
	rand.Seed(time.Now().UnixNano())
	errRate := parseErrorRate(os.Getenv("ERROR_RATE"))

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/metrics", handleMetrics)
	mux.HandleFunc("/shorten", handleShorten(errRate))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("api listening on :%s (error_rate=%.2f)", port, errRate)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func parseErrorRate(v string) float64 {
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(healthResp{Status: "ok"})
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP api_requests_total Total requests handled\n")
	fmt.Fprintf(w, "# TYPE api_requests_total counter\n")
	fmt.Fprintf(w, "api_requests_total %d\n", atomic.LoadUint64(&requestCount))
	fmt.Fprintf(w, "# HELP api_errors_total Total requests that returned an error\n")
	fmt.Fprintf(w, "# TYPE api_errors_total counter\n")
	fmt.Fprintf(w, "api_errors_total %d\n", atomic.LoadUint64(&errorCount))
}

// handleShorten fakes a link-shortener response. ERROR_RATE lets us inject
// synthetic failures for the canary/rollback demo without touching real traffic.
func handleShorten(errRate float64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hits := atomic.AddUint64(&requestCount, 1)

		url := r.URL.Query().Get("url")
		if url == "" {
			url = "https://sjsu.edu"
		}

		if errRate > 0 && rand.Float64() < errRate {
			atomic.AddUint64(&errorCount, 1)
			http.Error(w, "injected failure", http.StatusInternalServerError)
			return
		}

		resp := shortenResp{
			ShortCode:   fmt.Sprintf("cl%d", hits%9999),
			OriginalURL: url,
			Hits:        hits,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
