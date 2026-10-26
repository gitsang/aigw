package router

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/gitsang/aigw/internal/store"
)

func target(name, role string, weight int, healthy bool) store.RouteTarget {
	return store.RouteTarget{
		ID: name, Role: role, ProviderName: name, Weight: weight, Enabled: true,
		Health: &store.TargetHealth{Healthy: healthy},
	}
}

func rule(policy string, retries int) store.RoutingRule {
	return store.RoutingRule{
		VirtualModel: "deepseek-flash", RetryCount: retries, RetryPolicy: "fixed",
		FallbackPolicy: policy, RetryDelayMS: 0,
		Targets: []store.RouteTarget{
			target("provider1", "lb", 50, true),
			target("provider2", "lb", 50, true),
			target("provider3", "fallback", 100, true),
		},
	}
}

func TestFallbackFirstOrder(t *testing.T) {
	r := NewRunner()
	r.Rand = rand.New(rand.NewSource(1))
	var seen []string
	res := r.Execute(context.Background(), rule("fallback-first", 3), func(_ context.Context, t store.RouteTarget) AttemptOutcome {
		seen = append(seen, t.ProviderName)
		return AttemptOutcome{Retryable: true, StatusCode: 500}
	})
	if len(seen) != 9 {
		t.Fatalf("expected 9 attempts, got %d: %v", len(seen), seen)
	}
	if !res.FallbackUsed || len(res.Attempts) != 9 {
		t.Fatalf("unexpected result: %+v", res)
	}
	// each cycle must contain provider1, provider2 and provider3
	for cycle := 0; cycle < 3; cycle++ {
		chunk := seen[cycle*3 : cycle*3+3]
		set := map[string]bool{}
		for _, name := range chunk {
			set[name] = true
		}
		if !set["provider1"] || !set["provider2"] || !set["provider3"] {
			t.Fatalf("cycle %d incomplete: %v", cycle, chunk)
		}
	}
}

func TestRetryFirstOrder(t *testing.T) {
	r := NewRunner()
	r.Rand = rand.New(rand.NewSource(1))
	var seen []string
	r.Execute(context.Background(), rule("retry-first", 3), func(_ context.Context, t store.RouteTarget) AttemptOutcome {
		seen = append(seen, t.ProviderName)
		return AttemptOutcome{Retryable: true, StatusCode: 500}
	})
	if len(seen) != 9 {
		t.Fatalf("expected 9 attempts, got %d: %v", len(seen), seen)
	}
	first := seen[0]
	count := 0
	for _, name := range seen {
		if name == first {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("expected first route to be retried 3 times, got %v", seen)
	}
	if seen[6] != "provider3" || seen[7] != "provider3" || seen[8] != "provider3" {
		t.Fatalf("fallback should be last: %v", seen)
	}
}

func TestAllLBUnhealthyUsesFallbackOnly(t *testing.T) {
	r := NewRunner()
	rl := rule("fallback-first", 2)
	rl.Targets[0].Health.Healthy = false
	rl.Targets[1].Health.Healthy = false
	var seen []string
	res := r.Execute(context.Background(), rl, func(_ context.Context, t store.RouteTarget) AttemptOutcome {
		seen = append(seen, t.ProviderName)
		return AttemptOutcome{Success: true, StatusCode: 200}
	})
	if len(seen) != 1 || seen[0] != "provider3" || !res.FallbackUsed {
		t.Fatalf("expected only fallback, got %v (%+v)", seen, res)
	}
}

func TestStopsOnNonRetryable(t *testing.T) {
	r := NewRunner()
	rl := rule("fallback-first", 3)
	seen := 0
	res := r.Execute(context.Background(), rl, func(_ context.Context, t store.RouteTarget) AttemptOutcome {
		seen++
		return AttemptOutcome{StatusCode: 400}
	})
	if seen != 1 || res.StatusCode != 400 {
		t.Fatalf("expected one attempt, got %d %+v", seen, res)
	}
}

func TestBackoffRespectsContext(t *testing.T) {
	r := NewRunner()
	rl := rule("fallback-first", 2)
	rl.RetryDelayMS = 1000
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	r.Execute(ctx, rl, func(_ context.Context, t store.RouteTarget) AttemptOutcome {
		return AttemptOutcome{Retryable: true, StatusCode: 500}
	})
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("backoff did not respect context")
	}
}
