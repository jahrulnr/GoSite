// Command dev-receiver logs tier-0 webhook payloads for local testing.
//
//	go run ./dev-receiver
//	Listens on :9191, path /gosite
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := env("ADDR", ":9191")
	path := env("PATH", "/gosite")
	secret := os.Getenv("WEBHOOK_SECRET")

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		event := r.Header.Get("X-Gosite-Webhook-Event")
		gotSecret := r.Header.Get("X-Gosite-Webhook-Secret")
		if secret != "" && gotSecret != secret {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var pretty any
		if err := json.Unmarshal(body, &pretty); err != nil {
			log.Printf("[%s] %s %s raw=%s", event, r.Method, r.RemoteAddr, string(body))
		} else {
			enc, _ := json.MarshalIndent(pretty, "", "  ")
			log.Printf("[%s] %s %s\n%s", event, r.Method, r.RemoteAddr, enc)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("tier-0 dev receiver listening on http://127.0.0.1%s%s", addr, path)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
