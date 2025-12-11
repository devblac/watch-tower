package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/devblac/watch-tower/internal/sink"
	"github.com/devblac/watch-tower/internal/source/algorand"
	"github.com/devblac/watch-tower/internal/source/evm"
	"github.com/devblac/watch-tower/internal/storage"
)

// Runner wires sources, predicates, dedupe, and sinks for a single pass.
type Runner struct {
	store      *storage.Store
	sinks      map[string]sink.Sender
	rules      map[string]ruleExec
	evmScan    map[string]*evm.Scanner
	algoScan   map[string]*algorand.Scanner
	dryRun     bool
	nowFunc    func() time.Time
	targetFrom uint64
	targetTo   uint64
}

type Event struct {
	RuleID   string
	Chain    string
	SourceID string
	Height   uint64
	Hash     string
	TxHash   string
	LogIndex *uint
	AppID    uint64
	Args     map[string]any
}

type ruleExec struct {
	rule  config.Rule
	preds []Predicate
	ttl   time.Duration
}

// NewRunner builds a runner for the provided config and scanners.
func NewRunner(store *storage.Store, cfg *config.Config, evmScanners map[string]*evm.Scanner, algoScanners map[string]*algorand.Scanner, sinks map[string]sink.Sender, dryRun bool, from, to uint64) (*Runner, error) {
	rules := make(map[string]ruleExec, len(cfg.Rules))
	for _, r := range cfg.Rules {
		preds, err := CompilePredicates(r.Match.Where)
		if err != nil {
			return nil, fmt.Errorf("rule %s predicates: %w", r.ID, err)
		}
		var ttl time.Duration
		if r.Dedupe != nil && r.Dedupe.TTL != "" {
			if d, err := time.ParseDuration(r.Dedupe.TTL); err == nil {
				ttl = d
			}
		}
		rules[r.ID] = ruleExec{rule: r, preds: preds, ttl: ttl}
	}

	return &Runner{
		store:      store,
		sinks:      sinks,
		rules:      rules,
		evmScan:    evmScanners,
		algoScan:   algoScanners,
		dryRun:     dryRun,
		nowFunc:    time.Now,
		targetFrom: from,
		targetTo:   to,
	}, nil
}

// RunOnce processes one eligible block/round per source.
func (r *Runner) RunOnce(ctx context.Context) error {
	for id, sc := range r.evmScan {
		if r.targetTo > 0 {
			// stop if beyond target
			h, _, ok, err := r.store.GetCursor(ctx, id)
			if err != nil {
				return err
			}
			if ok && h >= r.targetTo {
				continue
			}
		}
		events, err := sc.ProcessNext(ctx)
		if err != nil {
			if err == evm.ErrReorgDetected {
				continue
			}
			return fmt.Errorf("evm source %s: %w", id, err)
		}
		evs := make([]Event, 0, len(events))
		for _, e := range events {
			evs = append(evs, Event{
				RuleID:   e.RuleID,
				Chain:    e.Chain,
				SourceID: e.SourceID,
				Height:   e.Height,
				Hash:     e.Hash,
				TxHash:   e.TxHash,
				LogIndex: e.LogIndex,
				AppID:    0,
				Args:     e.Args,
			})
		}
		if err := r.handleEvents(ctx, evs); err != nil {
			return err
		}
	}

	for id, sc := range r.algoScan {
		if r.targetTo > 0 {
			h, _, ok, err := r.store.GetCursor(ctx, id)
			if err != nil {
				return err
			}
			if ok && h >= r.targetTo {
				continue
			}
		}
		events, err := sc.ProcessNext(ctx)
		if err != nil {
			if err == algorand.ErrReorgDetected {
				continue
			}
			return fmt.Errorf("algorand source %s: %w", id, err)
		}
		evs := make([]Event, 0, len(events))
		for _, e := range events {
			evs = append(evs, Event{
				RuleID:   e.RuleID,
				Chain:    e.Chain,
				SourceID: e.SourceID,
				Height:   e.Height,
				Hash:     e.Hash,
				TxHash:   e.TxHash,
				AppID:    e.AppID,
				Args:     e.Args,
			})
		}
		if err := r.handleEvents(ctx, evs); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) handleEvents(ctx context.Context, events []Event) error {
	for _, ev := range events {
		exec, ok := r.rules[ev.RuleID]
		if !ok {
			continue
		}
		pass, err := allPredicates(exec.preds, ev.Args)
		if err != nil || !pass {
			continue
		}
		if exec.rule.Dedupe != nil {
			key := buildDedupeKey(exec.rule.Dedupe.Key, ev)
			now := r.nowFunc()
			isDup, err := r.store.IsDuplicate(ctx, key, now)
			if err != nil {
				return err
			}
			if isDup {
				continue
			}
			exp := now.Add(exec.ttl)
			if exec.ttl == 0 {
				exp = now.Add(24 * time.Hour)
			}
			if err := r.store.MarkDedupe(ctx, key, exp); err != nil {
				return err
			}
		}
		if r.dryRun {
			continue
		}
		for _, sinkID := range exec.rule.Sinks {
			s := r.sinks[sinkID]
			if s == nil {
				continue
			}
			if err := s.Send(ctx, toSinkPayload(ev, exec.rule.ID)); err != nil {
				return err
			}
		}
	}
	return nil
}

func allPredicates(preds []Predicate, args map[string]any) (bool, error) {
	for _, p := range preds {
		ok, err := p(args)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func buildDedupeKey(pattern string, ev Event) string {
	if pattern == "" {
		pattern = "txhash"
	}
	key := strings.ReplaceAll(pattern, "txhash", ev.TxHash)
	if ev.LogIndex != nil {
		key = strings.ReplaceAll(key, "logIndex", fmt.Sprintf("%d", *ev.LogIndex))
	}
	if ev.AppID != 0 {
		key = strings.ReplaceAll(key, "app_id", fmt.Sprintf("%d", ev.AppID))
	}
	return key
}

func toSinkPayload(ev Event, ruleID string) sink.EventPayload {
	return sink.EventPayload{
		RuleID:   ruleID,
		Chain:    ev.Chain,
		SourceID: ev.SourceID,
		Height:   ev.Height,
		Hash:     ev.Hash,
		TxHash:   ev.TxHash,
		LogIndex: ev.LogIndex,
		AppID:    ev.AppID,
		Args:     ev.Args,
	}
}
