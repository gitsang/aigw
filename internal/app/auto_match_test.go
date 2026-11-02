package app

import (
	"context"
	"testing"

	"github.com/gitsang/aigw/internal/router"
	"github.com/gitsang/aigw/internal/store"
)

func TestAutoMatchRuleExecutesSingleTarget(t *testing.T) {
	target := store.RuntimeTarget{
		TargetID: "pm-1", ProviderID: "p-1", ProviderName: "yllm",
		UpstreamModel: "deepseek/deepseek-flash",
	}
	rule := autoMatchRule("deepseek/deepseek-flash", target)

	if rule.Name != autoMatchRouteName || rule.VirtualModel != "deepseek/deepseek-flash" {
		t.Fatalf("unexpected rule identity: %+v", rule)
	}
	if len(rule.Targets) != 1 {
		t.Fatalf("expected one target, got %d", len(rule.Targets))
	}
	only := rule.Targets[0]
	if only.ProviderID != "p-1" || only.ProviderName != "yllm" ||
		only.UpstreamModel != "deepseek/deepseek-flash" || !only.Enabled {
		t.Fatalf("unexpected target: %+v", only)
	}

	// The runner must attempt exactly the auto-matched target once, because the
	// synthetic rule has a single target and a retry count of one.
	runner := router.NewRunner()
	var seen []string
	result := runner.Execute(t.Context(), rule, func(_ context.Context, rt store.RouteTarget) router.AttemptOutcome {
		seen = append(seen, rt.ProviderName)
		return router.AttemptOutcome{Success: true, StatusCode: 200}
	})
	if result.Err != nil || len(seen) != 1 || seen[0] != "yllm" {
		t.Fatalf("runner result = %+v, seen = %v", result, seen)
	}
}
