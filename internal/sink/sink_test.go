package sink

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlackSenderRendersTemplate(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		got = string(buf)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender, err := NewSlackSender(server.URL, "ALERT {{.RuleID}} {{.Chain}} {{short_addr .TxHash}}")
	if err != nil {
		t.Fatalf("sender: %v", err)
	}

	err = sender.Send(context.Background(), EventPayload{
		RuleID: "r1", Chain: "evm", TxHash: "0x1234567890abcdef",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	if got == "" || !contains(got, "ALERT r1 evm 0x1234") {
		t.Fatalf("unexpected payload: %s", got)
	}
}

func TestWebhookStatusFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	sender, err := NewWebhookSender(server.URL, http.MethodPost, "msg", nil)
	if err != nil {
		t.Fatalf("sender: %v", err)
	}
	err = sender.Send(context.Background(), EventPayload{RuleID: "r"})
	if err == nil {
		t.Fatalf("expected error on 502")
	}
}

func contains(s, substr string) bool { return strings.Contains(s, substr) }

