// Command testupstream starts a harmless local HTTP server used as the
// configured upstream for the demo. It listens on 127.0.0.1:8099 by default
// and returns a small JSON body so proxy behaviour can be observed.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := os.Getenv("GARUDA_TEST_UPSTREAM_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8099"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Upstream", "garudashield-test")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"upstream": "garudashield-test",
			"path":     r.URL.Path,
			"method":   r.Method,
			"host":     r.Host,
			"time":     time.Now().UTC(),
		})
	})
	log.Printf("test upstream listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
