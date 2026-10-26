// Package health periodically probes routing targets and persists their health.
package health

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gitsang/aigw/internal/store"
	"github.com/gitsang/aigw/internal/upstream"
)

type Prober struct {
	store       *store.Store
	client      *http.Client
	interval    time.Duration
	timeout     time.Duration
	concurrency int
	logf        func(format string, args ...any)
}

func New(store *store.Store, interval, timeout time.Duration, concurrency int, logf func(string, ...any)) *Prober {
	if interval <= 0 {
		interval = time.Minute
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if concurrency <= 0 {
		concurrency = 8
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Prober{
		store: store,
		client: &http.Client{Transport: &http.Transport{
			MaxIdleConns: 100, MaxIdleConnsPerHost: 20, IdleConnTimeout: 60 * time.Second,
		}},
		interval: interval, timeout: timeout, concurrency: concurrency, logf: logf,
	}
}

// Run performs an immediate check, then repeats once per interval.
func (p *Prober) Run(ctx context.Context) {
	p.CheckOnce(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.CheckOnce(ctx)
		}
	}
}

type probeResult struct {
	targetID   string
	healthy    bool
	latencyMS  int
	statusCode int
	errText    string
}

func (p *Prober) CheckOnce(ctx context.Context) {
	targets, err := p.store.ListHealthProbeTargets(ctx)
	if err != nil {
		p.logf("health: list targets: %v", err)
		return
	}
	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	results := make(chan probeResult, len(targets))
	for _, target := range targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			results <- p.probe(ctx, target)
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	for result := range results {
		if err := p.store.UpdateTargetHealth(ctx, result.targetID, result.healthy, result.latencyMS, result.statusCode, result.errText); err != nil {
			p.logf("health: update %s: %v", result.targetID, err)
			continue
		}
		if !result.healthy {
			p.logf("health: target %s unhealthy: status=%d error=%s", result.targetID, result.statusCode, result.errText)
		}
	}
}

func (p *Prober) probe(ctx context.Context, t store.HealthProbeTarget) probeResult {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	result := probeResult{targetID: t.TargetID}
	start := time.Now()

	path := t.UpstreamPath
	if t.HealthCheckURL != "" {
		path = t.HealthCheckURL
	}
	endpoint, err := upstream.BuildURL(t.BaseURL, path)
	if err != nil {
		result.errText = err.Error()
		result.latencyMS = int(time.Since(start).Milliseconds())
		return result
	}
	method := t.HealthCheckMethod
	if method == "" {
		method = http.MethodPost
	}
	var body io.Reader
	if method == http.MethodPost {
		payload, _ := json.Marshal(probeBody(t))
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		result.errText = err.Error()
		result.latencyMS = int(time.Since(start).Milliseconds())
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	upstream.ApplyAuth(req.Header, store.RuntimeTarget{
		AuthType: t.AuthType, APIKey: t.APIKey, APIType: t.APIType,
	})
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}
	resp, err := p.client.Do(req)
	result.latencyMS = int(time.Since(start).Milliseconds())
	if err != nil {
		result.errText = err.Error()
		return result
	}
	defer resp.Body.Close()
	result.statusCode = resp.StatusCode
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.errText = fmt.Sprintf("health check status %d: %s", resp.StatusCode, truncate(string(raw), 300))
		return result
	}
	if looksLikeError(raw) {
		result.errText = "health check response contains error: " + truncate(string(raw), 300)
		return result
	}
	result.healthy = true
	return result
}

func probeBody(t store.HealthProbeTarget) map[string]any {
	switch t.APIType {
	case store.APIResponses:
		return map[string]any{
			"model": t.UpstreamModel, "input": "ping", "max_output_tokens": 1, "stream": false,
		}
	case store.APIMessages:
		return map[string]any{
			"model": t.UpstreamModel, "max_tokens": 1, "stream": false,
			"messages": []map[string]any{{"role": "user", "content": "ping"}},
		}
	default:
		return map[string]any{
			"model": t.UpstreamModel, "max_tokens": 1, "stream": false,
			"messages": []map[string]any{{"role": "user", "content": "ping"}},
		}
	}
}

func looksLikeError(raw []byte) bool {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	if _, ok := m["error"]; ok {
		return true
	}
	if success, ok := m["success"].(bool); ok && !success {
		return true
	}
	return false
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
