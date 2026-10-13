package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hmcalister/Cirrus/internal/email"
	"github.com/hmcalister/Cirrus/internal/email/sender"
)

var _ sender.Sender = &RateLimitedSender{}

type RateLimitedSenderConfig struct {
	Next                 sender.Sender
	AddressBurst         int
	AddressInterval      time.Duration
	GlobalBurst          int
	GlobalInterval       time.Duration
	SweepCadenceDuration time.Duration
}

type RateLimitedSender struct {
	next sender.Sender

	// Enables testing of rate limiting
	now func() time.Time

	tokenBucketMutex      sync.Mutex
	addressTokenBucketMap map[string]*tokenBucket
	globalTokenBucket     *tokenBucket
	lastSweepTime         time.Time

	addressBurst         int
	addressInterval      time.Duration
	sweepCadenceDuration time.Duration
}

// Wrap a sender.Sender with rate limiting logic
func NewRateLimitedSender(cfg RateLimitedSenderConfig) (*RateLimitedSender, error) {
	if cfg.Next == nil {
		return nil, errors.New("next must be non-nil")
	}
	if cfg.AddressBurst <= 0 || cfg.GlobalBurst <= 0 {
		return nil, errors.New("burst must be positive integer")
	}
	if cfg.AddressInterval <= 0 || cfg.GlobalInterval <= 0 {
		return nil, errors.New("interval must positive")
	}
	if cfg.SweepCadenceDuration <= 0 {
		return nil, errors.New("sweep cadence must positive")
	}

	now := time.Now()
	return &RateLimitedSender{
		next:                  cfg.Next,
		now:                   time.Now,
		addressTokenBucketMap: make(map[string]*tokenBucket),
		globalTokenBucket:     newTokenBucket(cfg.GlobalBurst, cfg.GlobalInterval),
		lastSweepTime:         now,
		addressBurst:          cfg.AddressBurst,
		addressInterval:       cfg.AddressInterval,
		sweepCadenceDuration:  cfg.SweepCadenceDuration,
	}, nil
}

func (s *RateLimitedSender) Send(ctx context.Context, params sender.SendParams) error {
	if err := s.checkRateLimits(params.Recipients); err != nil {
		return err
	}
	return s.next.Send(ctx, params)
}

// Check the address rate limit (init if it exists) and the global rate limit.
// If both have a token, decrement both and return nil.
// If either has no tokens, return an error.
func (s *RateLimitedSender) checkRateLimits(r email.Recipients) error {
	now := s.now()

	allAddresses := make([]string, 0)
	allAddresses = append(allAddresses, r.ToRecipients...)
	allAddresses = append(allAddresses, r.CCRecipients...)
	allAddresses = append(allAddresses, r.BCCRecipients...)

	if len(allAddresses) == 0 {
		return errors.New("must have at least one recipient")
	}

	defer s.sweep(now)
	s.tokenBucketMutex.Lock()
	defer s.tokenBucketMutex.Unlock()

	// Keys in this list also track duplicate buckets
	spendBuckets := make(map[string]*tokenBucket)

	for _, address := range allAddresses {
		// Handle duplication of emails
		address = email.Normalize(address)
		if address == "" || spendBuckets[address] != nil {
			continue
		}

		bucket := s.addressTokenBucketMap[address]
		if bucket == nil {
			bucket = newTokenBucket(s.addressBurst, s.addressInterval)
		}
		spendBuckets[address] = bucket

		if wait := bucket.durationUntilNextToken(now); wait > 0 {
			return fmt.Errorf("%w (retry in %s)", ErrRateLimited, wait.Round(time.Second))
		}
	}
	if wait := s.globalTokenBucket.durationUntilNextToken(now); wait > 0 {
		return fmt.Errorf("%w: global limit (retry in %s)", ErrRateLimited, wait.Round(time.Second))
	}

	s.globalTokenBucket.tokens--
	for address, bucket := range spendBuckets {
		bucket.tokens--
		s.addressTokenBucketMap[address] = bucket
	}
	return nil
}

// Forget addresses that have been idle long enough to be back at full burst.
func (s *RateLimitedSender) sweep(now time.Time) {
	s.tokenBucketMutex.Lock()
	defer s.tokenBucketMutex.Unlock()

	if now.Sub(s.lastSweepTime) < s.sweepCadenceDuration {
		return
	}
	s.lastSweepTime = now
	for address, bucket := range s.addressTokenBucketMap {
		if now.Sub(bucket.last) >= time.Duration(s.addressBurst)*s.addressInterval {
			delete(s.addressTokenBucketMap, address)
		}
	}
}
