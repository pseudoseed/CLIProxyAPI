package auth

import (
	"context"
	"strconv"
	"strings"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
)

// SoonestResetSelector spends quota that resets soonest within the highest
// available priority tier. Unknown or expired observations use round robin.
// SessionAffinitySelector wraps this policy so active threads retain their account.
type SoonestResetSelector struct{ fallback RoundRobinSelector }

func (s *SoonestResetSelector) Pick(ctx context.Context, provider, model string, opts cliproxyexecutor.Options, auths []*Auth) (*Auth, error) {
	now := time.Now()
	available, err := getSelectorAvailableAuths(ctx, auths, provider, model, now)
	if err != nil {
		return nil, err
	}
	var earliest time.Time
	var preferred []*Auth
	for _, candidate := range available {
		reset := NextQuotaReset(candidate, now)
		if reset.IsZero() {
			continue
		}
		if earliest.IsZero() || reset.Before(earliest) {
			earliest = reset
			preferred = []*Auth{candidate}
		} else if reset.Equal(earliest) {
			preferred = append(preferred, candidate)
		}
	}
	if len(preferred) > 0 {
		available = preferred
	}
	return s.fallback.Pick(ctx, provider, model, opts, available)
}

// NextQuotaReset returns the earliest future subscription quota reset observed
// from upstream. Relative reset values are anchored to observation time, never
// request time. Expired and malformed signals cannot bias account selection.
func NextQuotaReset(auth *Auth, now time.Time) time.Time {
	if auth == nil {
		return time.Time{}
	}
	var earliest time.Time
	for name, value := range auth.Quota.Signals {
		name = strings.ToLower(name)
		var reset time.Time
		switch {
		case auth.Provider == "claude" && (name == "anthropic-ratelimit-unified-5h-reset" || name == "anthropic-ratelimit-unified-7d-reset"):
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				reset = time.Unix(seconds, 0)
			} else {
				reset, _ = time.Parse(time.RFC3339, value)
			}
		case auth.Provider == "codex" && (name == "x-codex-primary-reset-at" || name == "x-codex-secondary-reset-at"):
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				reset = time.Unix(seconds, 0)
			}
		case auth.Provider == "codex" && (name == "x-codex-primary-reset-after-seconds" || name == "x-codex-secondary-reset-after-seconds"):
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err == nil && seconds > 0 && seconds <= int64((365*24*time.Hour)/time.Second) && !auth.Quota.ObservedAt.IsZero() {
				reset = auth.Quota.ObservedAt.Add(time.Duration(seconds) * time.Second)
			}
		}
		if reset.After(now) && (earliest.IsZero() || reset.Before(earliest)) {
			earliest = reset
		}
	}
	return earliest
}
