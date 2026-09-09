// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

func TestAdaptiveLimiter_NewNilOnNonPositive(t *testing.T) {
	for _, rate := range []float64{0, -1} {
		if NewAdaptiveLimiter(rate) != nil {
			t.Errorf("NewAdaptiveLimiter(%v) should return nil", rate)
		}
	}
}

func TestAdaptiveLimiter_NilSafeMethods(t *testing.T) {
	var l *AdaptiveLimiter
	l.Wait()
	l.OnSuccess()
	l.OnRateLimit()
	if got := l.Rate(); got != 0 {
		t.Errorf("nil limiter Rate() = %v, want 0", got)
	}
}

func TestAdaptiveLimiter_RecoversAfterRateLimit(t *testing.T) {
	l := NewAdaptiveLimiter(8)
	l.OnRateLimit()
	backedOffRate := l.Rate()
	for range 100 {
		l.OnSuccess()
		if got := l.Rate(); got > 8 {
			t.Fatalf("recovery rate = %v, exceeds configured maximum 8", got)
		}
	}
	if got := l.Rate(); got <= backedOffRate {
		t.Errorf("rate after successes = %v, want recovery above %v", got, backedOffRate)
	}
}

func TestAdaptiveLimiter_SuccessesRespectConfiguredMaximum(t *testing.T) {
	for _, maximum := range []float64{0.25, 0.5, 2, 8} {
		l := NewAdaptiveLimiter(maximum)
		for success := range 100 {
			l.OnSuccess()
			if got := l.Rate(); got > maximum {
				t.Errorf("maximum %v: rate after %d successes = %v, exceeds configured maximum", maximum, success+1, got)
				break
			}
		}
	}
}

func TestAdaptiveLimiter_HalvesOnRateLimit(t *testing.T) {
	l := NewAdaptiveLimiter(8)
	l.OnRateLimit()
	if got := l.Rate(); got != 4 {
		t.Errorf("rate after OnRateLimit = %v, want 4", got)
	}
}

func TestAdaptiveLimiter_FloorsAtHalfRPS(t *testing.T) {
	l := NewAdaptiveLimiter(2)
	for range 10 {
		l.OnRateLimit()
	}
	if got := l.Rate(); got < 0.5 {
		t.Errorf("rate after repeated throttling = %v, want >= 0.5", got)
	}
}

func TestAdaptiveLimiter_BackoffAndRecoveryRespectConfiguredMaximum(t *testing.T) {
	for _, maximum := range []float64{0.25, 0.5, 2, 8} {
		l := NewAdaptiveLimiter(maximum)
		for cycle := range 10 {
			before := l.Rate()
			l.OnRateLimit()
			if got := l.Rate(); got <= 0 || got > before || got > maximum {
				t.Errorf("maximum %v, cycle %d: backoff changed rate from %v to %v", maximum, cycle, before, got)
				break
			}
			for range 100 {
				l.OnSuccess()
				if got := l.Rate(); got <= 0 || got > maximum {
					t.Fatalf("maximum %v, cycle %d: recovery rate = %v, want positive and <= maximum", maximum, cycle, got)
				}
			}
		}
	}
}

func TestAdaptiveLimiter_WaitEnforcesPacing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := NewAdaptiveLimiter(10)
		start := time.Now()
		l.Wait()
		if elapsed := time.Since(start); elapsed != 0 {
			t.Fatalf("first request waited %v, want immediate admission", elapsed)
		}
		l.Wait()
		if elapsed := time.Since(start); elapsed != 100*time.Millisecond {
			t.Errorf("second request waited %v, want 100ms", elapsed)
		}
	})
}

func TestAdaptiveLimiter_ConcurrentWaitsSharePacing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := NewAdaptiveLimiter(4)
		l.Wait()
		start := time.Now()
		const callers = 8
		admissions := make(chan time.Duration, callers)
		for range callers {
			go func() {
				l.Wait()
				admissions <- time.Since(start)
			}()
		}
		times := make([]time.Duration, 0, callers)
		for range callers {
			times = append(times, <-admissions)
		}
		slices.Sort(times)
		previous := time.Duration(0)
		for i, admittedAt := range times {
			if gap := admittedAt - previous; gap < 250*time.Millisecond {
				t.Errorf("request %d admitted at %v, only %v after previous request; want >= 250ms", i+1, admittedAt, gap)
			}
			previous = admittedAt
		}
	})
}

func TestAdaptiveLimiter_RateLimitFeedbackExtendsQueuedWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := NewAdaptiveLimiter(2)
		l.Wait()
		admitted := make(chan struct{})
		go func() {
			l.Wait()
			close(admitted)
		}()
		synctest.Wait()

		// The queued waiter initially observed a 500 ms interval. A 429
		// lowers the rate to 1 req/s while it is asleep, so it must recheck
		// and remain blocked until the extended interval has elapsed.
		l.OnRateLimit()
		time.Sleep(500 * time.Millisecond)
		select {
		case <-admitted:
			t.Fatal("queued waiter admitted before the feedback-adjusted interval")
		default:
		}
		time.Sleep(500 * time.Millisecond)
		select {
		case <-admitted:
		case <-time.After(time.Nanosecond):
			t.Fatal("queued waiter did not admit after the feedback-adjusted interval")
		}
	})
}
