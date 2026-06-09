package handler

import "testing"

func TestLoginLimiterBlocksAfterFiveFailures(t *testing.T) {
	limiter := newLoginLimiter()
	defer limiter.stop()

	ip := "192.0.2.1"
	for i := 0; i < 5; i++ {
		if !limiter.Allow(ip) {
			t.Fatalf("attempt %d blocked before reaching limit", i+1)
		}
		limiter.RecordFailure(ip)
	}

	if limiter.Allow(ip) {
		t.Fatalf("ip was allowed after five failures")
	}
}

func TestLoginLimiterRecordSuccessClearsFailures(t *testing.T) {
	limiter := newLoginLimiter()
	defer limiter.stop()

	ip := "192.0.2.2"
	for i := 0; i < 4; i++ {
		limiter.RecordFailure(ip)
	}
	limiter.RecordSuccess(ip)

	for i := 0; i < 4; i++ {
		if !limiter.Allow(ip) {
			t.Fatalf("attempt %d blocked after successful login", i+1)
		}
		limiter.RecordFailure(ip)
	}
	if !limiter.Allow(ip) {
		t.Fatalf("ip should allow four fresh failures after success")
	}
}
