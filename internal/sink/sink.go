package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"
)

// EventPayload is the data passed to sinks.
type EventPayload struct {
	RuleID   string
	Chain    string
	SourceID string
	Height   uint64
	Hash     string
	TxHash   string
	AppID    uint64
	LogIndex *uint
	Args     map[string]any
}

type Sender interface {
	Send(ctx context.Context, payload EventPayload) error
}

type httpSender struct {
	url     string
	method  string
	render  *template.Template
	client  *http.Client
	headers map[string]string
}

// NewWebhookSender builds a generic HTTP sink.
func NewWebhookSender(url, method, tmpl string, headers map[string]string) (Sender, error) {
	if url == "" {
		return nil, fmt.Errorf("webhook url required")
	}
	if method == "" {
		method = http.MethodPost
	}
	t, err := parseTemplate(tmpl)
	if err != nil {
		return nil, err
	}
	return &httpSender{
		url:     url,
		method:  strings.ToUpper(method),
		render:  t,
		client:  defaultClient(),
		headers: headers,
	}, nil
}

// NewSlackSender builds a Slack-compatible webhook sink.
func NewSlackSender(url, tmpl string) (Sender, error) {
	return NewWebhookSender(url, http.MethodPost, tmpl, map[string]string{
		"Content-Type": "application/json",
	})
}

// NewTeamsSender builds a Teams-compatible webhook sink.
func NewTeamsSender(url, tmpl string) (Sender, error) {
	// Teams accepts simple {text: "..."} payloads.
	return NewWebhookSender(url, http.MethodPost, tmpl, map[string]string{
		"Content-Type": "application/json",
	})
}

func (s *httpSender) Send(ctx context.Context, payload EventPayload) error {
	bodyStr, err := executeTemplate(s.render, payload)
	if err != nil {
		return err
	}
	reqBody, err := json.Marshal(map[string]string{
		"text": bodyStr,
	})
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, s.method, s.url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("sink http status %d", resp.StatusCode)
	}
	return nil
}

func parseTemplate(tmpl string) (*template.Template, error) {
	if tmpl == "" {
		tmpl = "ALERT {{.RuleID}} {{.Chain}} {{.TxHash}}"
	}
	funcs := template.FuncMap{
		"pretty_json": func(v any) string {
			out, _ := json.MarshalIndent(v, "", "  ")
			return string(out)
		},
		"short_addr": func(addr string) string {
			if len(addr) <= 10 {
				return addr
			}
			return addr[:6] + "..." + addr[len(addr)-4:]
		},
	}
	return template.New("msg").Funcs(funcs).Parse(tmpl)
}

func executeTemplate(t *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}

func defaultClient() *http.Client {
	return &http.Client{
		Timeout: 8 * time.Second,
	}
}

