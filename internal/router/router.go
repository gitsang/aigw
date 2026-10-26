// Package router resolves virtual models to weighted upstream targets and
// executes retry/fallback policies.
package router

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/gitsang/aigw/internal/store"
)

var ErrNoHealthyRoute = errors.New("no healthy route available for model")

type AttemptOutcome struct {
	Success    bool
	Retryable  bool
	StatusCode int
	Err        error
}

type AttemptLog struct {
	Target     store.RouteTarget
	Attempt    int
	Cycle      int
	Latency    time.Duration
	StatusCode int
	Error      string
	Fallback   bool
}

type ExecutionResult struct {
	Target       store.RouteTarget
	Attempts     []AttemptLog
	FallbackUsed bool
	Err          error
	StatusCode   int
}

type Executor func(ctx context.Context, target store.RouteTarget) AttemptOutcome

type Runner struct {
	Rand *rand.Rand
}

func NewRunner() *Runner {
	return &Runner{Rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// BuildRoutes separates enabled targets into healthy LBs and ordered fallbacks.
func BuildRoutes(rule store.RoutingRule) (lb, fallback []store.RouteTarget) {
	for _, t := range rule.Targets {
		if !t.Enabled {
			continue
		}
		healthy := true
		if t.Health != nil {
			healthy = t.Health.Healthy
		}
		if t.Role == store.RoleFallback {
			fallback = append(fallback, t)
			continue
		}
		if healthy {
			lb = append(lb, t)
		}
	}
	sort.SliceStable(fallback, func(i, j int) bool {
		if fallback[i].Priority == fallback[j].Priority {
			return fallback[i].Weight > fallback[j].Weight
		}
		return fallback[i].Priority > fallback[j].Priority
	})
	return lb, fallback
}

// WeightedOrder returns a random weighted permutation of routes.
func (r *Runner) WeightedOrder(routes []store.RouteTarget) []store.RouteTarget {
	out := append([]store.RouteTarget(nil), routes...)
	if len(out) <= 1 {
		return out
	}
	// Exponential race: key = -ln(U)/weight; smaller key wins.
	type scored struct {
		t   store.RouteTarget
		key float64
	}
	scoredRoutes := make([]scored, 0, len(out))
	for _, t := range out {
		w := float64(t.Weight)
		if w <= 0 {
			w = 1
		}
		u := r.Rand.Float64()
		if u <= 0 {
			u = 1e-12
		}
		scoredRoutes = append(scoredRoutes, scored{t: t, key: -log(u) / w})
	}
	sort.SliceStable(scoredRoutes, func(i, j int) bool { return scoredRoutes[i].key < scoredRoutes[j].key })
	for i := range scoredRoutes {
		out[i] = scoredRoutes[i].t
	}
	return out
}

// Execute runs the configured retry and fallback policy.
func (r *Runner) Execute(ctx context.Context, rule store.RoutingRule, exec Executor) ExecutionResult {
	lb, fallback := BuildRoutes(rule)
	res := ExecutionResult{StatusCode: 502}
	if len(lb) == 0 && len(fallback) == 0 {
		res.Err = ErrNoHealthyRoute
		return res
	}

	retries := rule.RetryCount
	if retries < 1 {
		retries = 1
	}
	baseDelay := time.Duration(rule.RetryDelayMS) * time.Millisecond
	maxBackoff := time.Duration(rule.MaxBackoffMS) * time.Millisecond
	if maxBackoff <= 0 {
		maxBackoff = 5 * time.Second
	}

	attemptNo := 0
	runOne := func(t store.RouteTarget, cycle int, fallbackRoute bool) bool {
		if err := ctx.Err(); err != nil {
			res.Err = err
			return true
		}
		attemptNo++
		start := time.Now()
		outcome := exec(ctx, t)
		if fallbackRoute {
			res.FallbackUsed = true
		}
		entry := AttemptLog{
			Target: t, Attempt: attemptNo, Cycle: cycle,
			Latency: time.Since(start), StatusCode: outcome.StatusCode, Fallback: fallbackRoute,
		}
		if outcome.Err != nil {
			entry.Error = outcome.Err.Error()
		}
		res.Attempts = append(res.Attempts, entry)
		res.StatusCode = outcome.StatusCode
		if outcome.Success {
			res.Target = t
			res.Err = nil
			return true
		}
		res.Err = outcome.Err
		if res.Err == nil {
			res.Err = fmt.Errorf("upstream returned status %d", outcome.StatusCode)
		}
		if !outcome.Retryable {
			return true
		}
		return false
	}

	delayFor := func(attempt int) {
		if baseDelay <= 0 {
			return
		}
		d := baseDelay
		if rule.RetryPolicy == "exponential" && attempt > 1 {
			d = baseDelay * time.Duration(1<<min(attempt-1, 16))
		}
		if d > maxBackoff {
			d = maxBackoff
		}
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
	}

	switch rule.FallbackPolicy {
	case "retry-first":
		all := append(append([]store.RouteTarget{}, r.WeightedOrder(lb)...), fallback...)
		if len(lb) == 0 {
			all = fallback
		}
		for _, t := range all {
			isFallback := t.Role == store.RoleFallback
			for i := 0; i < retries; i++ {
				if runOne(t, i+1, isFallback) {
					return res
				}
				if i < retries-1 {
					delayFor(i + 1)
				}
			}
		}
	default: // fallback-first: provider1, provider2, ..., fallback, repeated retryCount cycles
		for cycle := 1; cycle <= retries; cycle++ {
			order := r.WeightedOrder(lb)
			order = append(order, fallback...)
			if len(lb) == 0 {
				order = fallback
			}
			for i, t := range order {
				if runOne(t, cycle, t.Role == store.RoleFallback) {
					return res
				}
				if i < len(order)-1 {
					delayFor(cycle)
				}
			}
		}
	}
	if res.Err == nil {
		res.Err = errors.New("all attempts failed")
	}
	return res
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func log(v float64) float64 {
	// avoid importing math solely for Log? keep math import explicit below.
	return mathLog(v)
}
