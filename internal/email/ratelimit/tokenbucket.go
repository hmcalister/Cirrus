package ratelimit

import "time"

// Token Bucket algorithm.
// The bucket constantly fills with tokens until a cap `burst` is reached
// A new token is available every `interval`. A token is removed to send an email.
type tokenBucket struct {
	tokens float64
	last   time.Time

	burst    int
	interval time.Duration
}

// Make a new token bucket.
// This is an internal method, so no validation is done on burst or interval.
func newTokenBucket(burst int, interval time.Duration) *tokenBucket {
	return &tokenBucket{
		tokens:   float64(burst),
		last:     time.Now(),
		burst:    burst,
		interval: interval,
	}
}

// Check if a token is available.
//
// Returns the duration until the next token is available.
// If zero is returned, a token is available right now.
func (b *tokenBucket) durationUntilNextToken(now time.Time) time.Duration {
	// Elapsed time is clamped at zero so a non-monotonic or backwards clock
	// never removes tokens.
	elapsed := max(now.Sub(b.last), 0)

	// set tokens to current tokens + some amount based on when `wait` last called.
	// min with burst sets a cap of burst.
	b.tokens = min(float64(b.burst), b.tokens+float64(elapsed)/float64(b.interval))

	b.last = now
	if b.tokens >= 1 {
		return 0
	}
	return time.Duration((1 - b.tokens) * float64(b.interval))
}
