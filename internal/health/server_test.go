package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		checker  Checker
		wantCode int
		wantDB   string
		wantRPC  string
	}{
		{
			name: "all_ok",
			checker: Checker{
				DBPing:  func(ctx context.Context) error { return nil },
				RPCPing: func(ctx context.Context) error { return nil },
			},
			wantCode: http.StatusOK,
			wantDB:   "ok",
			wantRPC:  "ok",
		},
		{
			name: "db_fail",
			checker: Checker{
				DBPing:  func(ctx context.Context) error { return context.DeadlineExceeded },
				RPCPing: func(ctx context.Context) error { return nil },
			},
			wantCode: http.StatusServiceUnavailable,
			wantDB:   "fail",
			wantRPC:  "ok",
		},
		{
			name: "rpc_fail",
			checker: Checker{
				DBPing:  func(ctx context.Context) error { return nil },
				RPCPing: func(ctx context.Context) error { return context.DeadlineExceeded },
			},
			wantCode: http.StatusServiceUnavailable,
			wantDB:   "ok",
			wantRPC:  "fail",
		},
		{
			name: "both_fail",
			checker: Checker{
				DBPing:  func(ctx context.Context) error { return context.DeadlineExceeded },
				RPCPing: func(ctx context.Context) error { return context.DeadlineExceeded },
			},
			wantCode: http.StatusServiceUnavailable,
			wantDB:   "fail",
			wantRPC:  "fail",
		},
		{
			name: "no_checkers",
			checker: Checker{
				DBPing:  nil,
				RPCPing: nil,
			},
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := Serve(":0", tt.checker)
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = Shutdown(ctx, srv)
			}()

			time.Sleep(50 * time.Millisecond)

			req := httptest.NewRequest(http.MethodGet, "http://localhost/healthz", nil)
			w := httptest.NewRecorder()

			srv.Handler.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("status code = %d, want %d", w.Code, tt.wantCode)
			}

			var resp map[string]string
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if resp["status"] != "ok" {
				t.Errorf("status = %q, want ok", resp["status"])
			}

			if tt.wantDB != "" && resp["db"] != tt.wantDB {
				t.Errorf("db = %q, want %q", resp["db"], tt.wantDB)
			}
			if tt.wantRPC != "" && resp["rpc"] != tt.wantRPC {
				t.Errorf("rpc = %q, want %q", resp["rpc"], tt.wantRPC)
			}
		})
	}
}
