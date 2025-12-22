package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Checker struct {
	DBPing  func(ctx context.Context) error
	RPCPing func(ctx context.Context) error
}

// Serve starts a minimal /healthz handler.
func Serve(addr string, checker Checker) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		status := map[string]string{"status": "ok"}
		code := http.StatusOK

		if checker.DBPing != nil {
			if err := checker.DBPing(ctx); err != nil {
				status["db"] = "fail"
				code = http.StatusServiceUnavailable
			} else {
				status["db"] = "ok"
			}
		}
		if checker.RPCPing != nil {
			if err := checker.RPCPing(ctx); err != nil {
				status["rpc"] = "fail"
				code = http.StatusServiceUnavailable
			} else {
				status["rpc"] = "ok"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(status)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	go func() { _ = srv.ListenAndServe() }()
	return srv
}

// Shutdown gracefully shuts down the health server.
func Shutdown(ctx context.Context, srv *http.Server) error {
	return srv.Shutdown(ctx)
}
