package auth

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
)

func TestNextQuotaReset(t *testing.T) {
	now := time.Unix(2000000000, 0)
	for _, tc := range []struct {
		name, provider, header, value string
		observed                      time.Time
		want                          time.Time
	}{
		{"claude", "claude", "Anthropic-Ratelimit-Unified-5h-Reset", "2000000300", now, now.Add(5 * time.Minute)},
		{"codex absolute", "codex", "X-Codex-Primary-Reset-At", "2000000600", now, now.Add(10 * time.Minute)},
		{"relative anchored", "codex", "X-Codex-Primary-Reset-After-Seconds", "600", now.Add(-time.Minute), now.Add(9 * time.Minute)},
		{"expired", "codex", "X-Codex-Primary-Reset-At", "1999999999", now, time.Time{}},
		{"malformed", "codex", "X-Codex-Primary-Reset-At", "invalid", now, time.Time{}},
		{"missing observation", "codex", "X-Codex-Primary-Reset-After-Seconds", "600", time.Time{}, time.Time{}},
		{"overflow", "codex", "X-Codex-Primary-Reset-After-Seconds", "9223372036854775807", now, time.Time{}},
		{"unrelated window", "codex", "X-Codex-Code-Review-Primary-Reset-At", "2000000600", now, time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &Auth{Provider: tc.provider, Quota: QuotaState{ObservedAt: tc.observed, Signals: map[string]string{tc.header: tc.value}}}
			if got := NextQuotaReset(a, now); !got.Equal(tc.want) {
				t.Fatalf("reset = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSoonestResetPriorityAffinityAndFailover(t *testing.T) {
	now := time.Now()
	a := &Auth{ID: "a", Provider: "codex", Quota: QuotaState{Signals: map[string]string{"X-Codex-Primary-Reset-At": strconv.FormatInt(now.Add(time.Hour).Unix(), 10)}}}
	b := &Auth{ID: "b", Provider: "codex", Quota: QuotaState{Signals: map[string]string{"X-Codex-Primary-Reset-At": strconv.FormatInt(now.Add(2*time.Hour).Unix(), 10)}}}
	selector := NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{Fallback: &SoonestResetSelector{}, TTL: time.Hour})
	defer selector.Stop()
	opts := cliproxyexecutor.Options{Metadata: map[string]any{cliproxyexecutor.DerivedSessionIDMetadataKey: "thread"}}
	pick := func(want string) {
		t.Helper()
		got, err := selector.Pick(context.Background(), "codex", "", opts, []*Auth{a, b})
		if err != nil || got.ID != want {
			t.Fatalf("pick = %v, %v; want %s", got, err, want)
		}
	}
	pick("a")
	b.Attributes = map[string]string{"priority": "10"}
	pick("a") // Existing threads stay on the available account despite a priority change.
	opts.Metadata[cliproxyexecutor.DerivedSessionIDMetadataKey] = "new-thread"
	pick("b")
	b.Disabled = true
	pick("a")
}

func TestSoonestResetUnknownUsesRoundRobin(t *testing.T) {
	s := &SoonestResetSelector{}
	for _, want := range []string{"a", "b", "a"} {
		got, err := s.Pick(context.Background(), "codex", "", cliproxyexecutor.Options{}, []*Auth{{ID: "a"}, {ID: "b"}})
		if err != nil || got.ID != want {
			t.Fatalf("pick = %v, %v, want %s", got, err, want)
		}
	}
}

func TestWebsocketSubscriptionDefaultsAndOverrides(t *testing.T) {
	a := &Auth{Provider: "codex", Attributes: map[string]string{AttributeAuthKind: AuthKindOAuth}}
	if !WebsocketsEnabled(a) {
		t.Fatal("OAuth should default to websocket")
	}
	a.Metadata = map[string]any{"websockets": false}
	if WebsocketsEnabled(a) {
		t.Fatal("explicit opt out ignored")
	}
	a.Attributes["websockets"] = "true"
	if !WebsocketsEnabled(a) {
		t.Fatal("attribute override ignored")
	}
	a = &Auth{Provider: "codex", Attributes: map[string]string{AttributeAPIKey: "test"}}
	if WebsocketsEnabled(a) {
		t.Fatal("API key transport default changed")
	}
}

func TestSoonestResetOrdersByResetBeforeID(t *testing.T) {
	now := time.Now()
	a := &Auth{ID: "a", Provider: "claude", Quota: QuotaState{Signals: map[string]string{"Anthropic-Ratelimit-Unified-5h-Reset": strconv.FormatInt(now.Add(2*time.Hour).Unix(), 10)}}}
	b := &Auth{ID: "b", Provider: "claude", Quota: QuotaState{Signals: map[string]string{"Anthropic-Ratelimit-Unified-5h-Reset": strconv.FormatInt(now.Add(time.Hour).Unix(), 10)}}}
	s := &SoonestResetSelector{}
	for i := 0; i < 3; i++ {
		got, err := s.Pick(context.Background(), "claude", "", cliproxyexecutor.Options{}, []*Auth{a, b})
		if err != nil || got != b {
			t.Fatalf("pick=%v, %v, want earliest reset b", got, err)
		}
	}
	b.Disabled = true
	got, err := s.Pick(context.Background(), "claude", "", cliproxyexecutor.Options{}, []*Auth{a, b})
	if err != nil || got != a {
		t.Fatalf("disabled account selected: %v, %v", got, err)
	}
}

func TestManagerSoonestResetPreservesRequestedProviderAndModel(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	const model = "claude-reset-routing-test"
	selector := NewSessionAffinitySelectorWithConfig(SessionAffinityConfig{Fallback: &SoonestResetSelector{}, TTL: time.Hour})
	defer selector.Stop()
	manager := NewManager(nil, selector, nil)
	manager.RegisterExecutor(schedulerProviderTestExecutor{provider: "claude"})
	manager.RegisterExecutor(schedulerProviderTestExecutor{provider: "codex"})
	reg := registry.GetGlobalRegistry()
	for _, account := range []struct {
		id, provider, model string
		reset               time.Duration
		used                string
	}{
		{"claude-sub-1", "claude", model, 48 * time.Hour, "0.92"},
		{"claude-sub-2", "claude", model, 8 * time.Hour, "0.90"},
		{"codex-sub", "codex", "codex-reset-routing-test", time.Hour, "75"},
		{"claude-other-model", "claude", "claude-other-reset-routing-test", 2 * time.Hour, "0.50"},
	} {
		id := t.Name() + "/" + account.id
		resetHeader, usedHeader := "anthropic-ratelimit-unified-7d-reset", "anthropic-ratelimit-unified-7d-utilization"
		if account.provider == "codex" {
			resetHeader, usedHeader = "x-codex-primary-reset-at", "x-codex-primary-used-percent"
		}
		candidate := &Auth{
			ID: id, Provider: account.provider, Status: StatusActive,
			Quota: QuotaState{ObservedAt: now, Signals: map[string]string{
				resetHeader: strconv.FormatInt(now.Add(account.reset).Unix(), 10),
				usedHeader:  account.used,
			}},
		}
		reg.RegisterClient(id, account.provider, []*registry.ModelInfo{{ID: account.model}})
		t.Cleanup(func() { reg.UnregisterClient(id) })
		if _, errRegister := manager.Register(ctx, candidate); errRegister != nil {
			t.Fatal(errRegister)
		}
	}
	for _, mixed := range []bool{false, true} {
		opts := cliproxyexecutor.Options{Metadata: map[string]any{cliproxyexecutor.DerivedSessionIDMetadataKey: "reset-routing-thread"}}
		tried := make(map[string]struct{})
		pick := func() (*Auth, error) {
			if mixed {
				// Even a provider pool containing Codex must retain the requested model.
				selected, _, _, errPick := manager.pickNextMixed(ctx, []string{"claude", "codex"}, model, opts, tried)
				return selected, errPick
			}
			selected, _, errPick := manager.pickNext(ctx, "claude", model, opts, tried)
			return selected, errPick
		}
		for _, want := range []string{"claude-sub-2", "claude-sub-1"} {
			selected, errPick := pick()
			if errPick != nil {
				t.Fatalf("mixed=%v: %v", mixed, errPick)
			}
			if selected.ID != t.Name()+"/"+want || selected.Provider != "claude" {
				t.Fatalf("mixed=%v: selected %s via %s, want %s via claude", mixed, selected.ID, selected.Provider, want)
			}
			tried[selected.ID] = struct{}{}
		}
		if selected, errPick := pick(); errPick == nil || selected != nil {
			t.Fatalf("mixed=%v: exhausted matching accounts should fail, got %v, %v", mixed, selected, errPick)
		}
	}
}
