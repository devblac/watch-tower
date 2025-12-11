package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config holds the YAML configuration.
type Config struct {
	Version int          `yaml:"version"`
	Global  GlobalConfig `yaml:"global"`
	Sources []Source     `yaml:"sources"`
	Rules   []Rule       `yaml:"rules"`
	Sinks   []Sink       `yaml:"sinks"`
}

type GlobalConfig struct {
	DBPath        string            `yaml:"db_path"`
	Confirmations map[string]uint64 `yaml:"confirmations"`
}

type Source struct {
	ID         string   `yaml:"id"`
	Type       string   `yaml:"type"`
	RPCURL     string   `yaml:"rpc_url"`
	StartBlock string   `yaml:"start_block"`
	ABIDirs    []string `yaml:"abi_dirs"`

	AlgodURL   string `yaml:"algod_url"`
	IndexerURL string `yaml:"indexer_url"`
	StartRound string `yaml:"start_round"`
}

type MatchSpec struct {
	Type     string   `yaml:"type"`
	Contract string   `yaml:"contract"`
	Event    string   `yaml:"event"`
	AppID    uint64   `yaml:"app_id"`
	Where    []string `yaml:"where"`
}

type Dedupe struct {
	Key string `yaml:"key"`
	TTL string `yaml:"ttl"`
}

type Rule struct {
	ID     string    `yaml:"id"`
	Source string    `yaml:"source"`
	Match  MatchSpec `yaml:"match"`
	Sinks  []string  `yaml:"sinks"`
	Dedupe *Dedupe   `yaml:"dedupe,omitempty"`
}

type Sink struct {
	ID         string `yaml:"id"`
	Type       string `yaml:"type"`
	WebhookURL string `yaml:"webhook_url"`
	Template   string `yaml:"template"`
	URL        string `yaml:"url"`
	Method     string `yaml:"method"`
}

var envPattern = regexp.MustCompile(`\${([A-Za-z_][A-Za-z0-9_]*)}`)

// Load reads, interpolates env vars, parses YAML, and validates.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("config path is required")
	}

	if err := loadDotEnv(path); err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	interpolated, err := interpolateEnv(string(raw))
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(interpolated), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func loadDotEnv(configPath string) error {
	envPath := filepath.Join(filepath.Dir(configPath), ".env")
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			return fmt.Errorf("load .env: %w", err)
		}
	}
	return nil
}

func interpolateEnv(input string) (string, error) {
	missing := []string{}
	out := envPattern.ReplaceAllStringFunc(input, func(match string) string {
		name := envPattern.FindStringSubmatch(match)[1]
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		missing = append(missing, name)
		return match
	})

	if len(missing) > 0 {
		return "", fmt.Errorf("missing environment variables: %s", strings.Join(dedup(missing), ", "))
	}
	return out, nil
}

// Validate performs small, direct schema checks.
func (c *Config) Validate() error {
	if c.Version == 0 {
		return errors.New("version is required")
	}
	if len(c.Sources) == 0 {
		return errors.New("at least one source is required")
	}
	if len(c.Sinks) == 0 {
		return errors.New("at least one sink is required")
	}
	if len(c.Rules) == 0 {
		return errors.New("at least one rule is required")
	}

	sourceIDs := map[string]struct{}{}
	for _, s := range c.Sources {
		if _, exists := sourceIDs[s.ID]; exists {
			return fmt.Errorf("duplicate source id: %s", s.ID)
		}
		sourceIDs[s.ID] = struct{}{}
		if err := s.Validate(); err != nil {
			return fmt.Errorf("source %s: %w", s.ID, err)
		}
	}

	sinkIDs := map[string]*Sink{}
	for i := range c.Sinks {
		s := &c.Sinks[i]
		if _, exists := sinkIDs[s.ID]; exists {
			return fmt.Errorf("duplicate sink id: %s", s.ID)
		}
		sinkIDs[s.ID] = s
		if err := s.Validate(); err != nil {
			return fmt.Errorf("sink %s: %w", s.ID, err)
		}
	}

	for _, r := range c.Rules {
		if err := r.Validate(sourceIDs, sinkIDs); err != nil {
			return fmt.Errorf("rule %s: %w", r.ID, err)
		}
	}

	return nil
}

func (s *Source) Validate() error {
	if s.ID == "" {
		return errors.New("id is required")
	}
	switch strings.ToLower(s.Type) {
	case "evm":
		if s.RPCURL == "" {
			return errors.New("rpc_url is required for evm sources")
		}
	case "algorand":
		if s.AlgodURL == "" || s.IndexerURL == "" {
			return errors.New("algod_url and indexer_url are required for algorand sources")
		}
	default:
		return fmt.Errorf("unsupported source type: %s", s.Type)
	}
	return nil
}

func (r *Rule) Validate(sourceIDs map[string]struct{}, sinkIDs map[string]*Sink) error {
	if r.ID == "" {
		return errors.New("id is required")
	}
	if r.Source == "" {
		return errors.New("source is required")
	}
	if _, ok := sourceIDs[r.Source]; !ok {
		return fmt.Errorf("unknown source: %s", r.Source)
	}

	if len(r.Sinks) == 0 {
		return errors.New("at least one sink is required")
	}
	for _, sinkID := range r.Sinks {
		if _, ok := sinkIDs[sinkID]; !ok {
			return fmt.Errorf("unknown sink: %s", sinkID)
		}
	}

	if r.Match.Type == "" {
		return errors.New("match.type is required")
	}
	switch strings.ToLower(r.Match.Type) {
	case "log":
		if r.Match.Contract == "" {
			return errors.New("match.contract is required for log match")
		}
		if r.Match.Event == "" {
			return errors.New("match.event is required for log match")
		}
	case "app_call":
		if r.Match.AppID == 0 {
			return errors.New("match.app_id is required for app_call match")
		}
	default:
		return fmt.Errorf("unsupported match.type: %s", r.Match.Type)
	}

	if r.Dedupe != nil {
		if r.Dedupe.Key == "" || r.Dedupe.TTL == "" {
			return errors.New("dedupe.key and dedupe.ttl are required when dedupe is set")
		}
	}

	return nil
}

func (s *Sink) Validate() error {
	if s.ID == "" {
		return errors.New("id is required")
	}
	if s.Type == "" {
		return errors.New("type is required")
	}

	switch strings.ToLower(s.Type) {
	case "slack", "teams":
		if s.WebhookURL == "" {
			return errors.New("webhook_url is required for slack/teams sinks")
		}
	case "webhook":
		if s.URL == "" {
			return errors.New("url is required for webhook sink")
		}
		if s.Method == "" {
			s.Method = "POST"
		}
	default:
		return fmt.Errorf("unsupported sink type: %s", s.Type)
	}
	return nil
}

func dedup(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
