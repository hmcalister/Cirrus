package ratelimit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hmcalister/Cirrus/internal/email"
	"github.com/hmcalister/Cirrus/internal/email/sender"
)

// mockSender records every call and optionally returns a fixed error.
type mockSender struct {
	mu    sync.Mutex
	calls []sender.SendParams
	err   error
}

func (s *mockSender) Send(_ context.Context, params sender.SendParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, params)
	return s.err
}

func (s *mockSender) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func makeSenderParams(addrs ...string) sender.SendParams {
	return sender.SendParams{
		Subject: "subject", Body: "body",
		ToRecipients: addrs,
	}
}

// baseConfig returns a valid config with the required positive intervals so
// individual tests only need to override the fields they exercise.
func baseConfig(next sender.Sender) RateLimitedSenderConfig {
	return RateLimitedSenderConfig{
		Next:                 next,
		AddressBurst:         1,
		AddressInterval:      time.Minute,
		GlobalBurst:          1,
		GlobalInterval:       time.Minute,
		SweepCadenceDuration: time.Minute,
	}
}

// newTestSender builds a RateLimitedSender with a controllable clock.
func newTestSender(t *testing.T, cfg RateLimitedSenderConfig) (*RateLimitedSender, *time.Time) {
	t.Helper()
	s, err := NewRateLimitedSender(cfg)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.now = func() time.Time { return now }
	return s, &now
}

func TestNewRateLimitedSenderValidation(t *testing.T) {
	next := &mockSender{}
	tests := []struct {
		name string
		cfg  RateLimitedSenderConfig
	}{
		{"nil next", baseConfig(nil)},
		{"zero address burst", func() RateLimitedSenderConfig { c := baseConfig(next); c.AddressBurst = 0; return c }()},
		{"negative global burst", func() RateLimitedSenderConfig { c := baseConfig(next); c.GlobalBurst = -1; return c }()},
		{"zero address interval", func() RateLimitedSenderConfig { c := baseConfig(next); c.AddressInterval = 0; return c }()},
		{"negative address interval", func() RateLimitedSenderConfig { c := baseConfig(next); c.AddressInterval = -time.Second; return c }()},
		{"zero global interval", func() RateLimitedSenderConfig { c := baseConfig(next); c.GlobalInterval = 0; return c }()},
		{"negative global interval", func() RateLimitedSenderConfig { c := baseConfig(next); c.GlobalInterval = -time.Second; return c }()},
		{"zero sweep cadence", func() RateLimitedSenderConfig { c := baseConfig(next); c.SweepCadenceDuration = 0; return c }()},
		{"negative sweep cadence", func() RateLimitedSenderConfig { c := baseConfig(next); c.SweepCadenceDuration = -time.Second; return c }()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewRateLimitedSender(tc.cfg); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestSendDelegatesUnderLimit(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.AddressBurst = 5
	cfg.GlobalBurst = 5
	s, _ := newTestSender(t, cfg)

	params := makeSenderParams("recipient@example.com")
	if err := s.Send(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	if next.callCount() != 1 {
		t.Errorf("next called %d times, want 1", next.callCount())
	}
	if next.calls[0].Subject != params.Subject || next.calls[0].ToRecipients[0] != params.ToRecipients[0] {
		t.Errorf("forwarded params = %+v", next.calls[0])
	}
}

func TestSendPropagatesNextError(t *testing.T) {
	next := &mockSender{err: email.ErrDelivery}
	cfg := baseConfig(next)
	cfg.AddressBurst = 5
	cfg.GlobalBurst = 5
	s, _ := newTestSender(t, cfg)

	err := s.Send(context.Background(), makeSenderParams("recipient@example.com"))
	if !errors.Is(err, email.ErrDelivery) {
		t.Errorf("err = %v, want ErrDelivery", err)
	}
}

func TestAddressBurstLimit(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.AddressBurst = 2
	cfg.GlobalBurst = 100
	s, _ := newTestSender(t, cfg)

	for i := range 2 {
		if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	err := s.Send(context.Background(), makeSenderParams("recipient@example.com"))
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if next.callCount() != 2 {
		t.Errorf("next called %d times, want 2", next.callCount())
	}
}

func TestGlobalBurstLimit(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.AddressBurst = 100
	cfg.GlobalBurst = 2
	s, _ := newTestSender(t, cfg)

	if err := s.Send(context.Background(), makeSenderParams("recipient1@example.com")); err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), makeSenderParams("recipient2@example.com")); err != nil {
		t.Fatal(err)
	}

	err := s.Send(context.Background(), makeSenderParams("recipient3@example.com"))
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
}

func TestPerAddressIsolation(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.AddressBurst = 1
	cfg.GlobalBurst = 100
	s, _ := newTestSender(t, cfg)

	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
	if err := s.Send(context.Background(), makeSenderParams("recipient2@example.com")); err != nil {
		t.Errorf("b send: %v, want nil", err)
	}
}

func TestNormalizationSharesBucket(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.GlobalBurst = 100
	s, _ := newTestSender(t, cfg)

	if err := s.Send(context.Background(), makeSenderParams("Recipient+tag@Example.com")); err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

func TestDuplicateRecipientsConsumeOneToken(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.GlobalBurst = 100
	s, _ := newTestSender(t, cfg)

	params := sender.SendParams{
		Subject: "subject", Body: "body",
		ToRecipients: []string{"recipient@example.com", "recipient@example.com"},
	}
	if err := s.Send(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

func TestCCAndBCCAreRateLimited(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.GlobalBurst = 100
	s, _ := newTestSender(t, cfg)

	params := sender.SendParams{
		Subject: "subject", Body: "body",
		ToRecipients:  []string{"to@example.com"},
		CCRecipients:  []string{"cc@example.com"},
		BCCRecipients: []string{"bcc@example.com"},
	}
	if err := s.Send(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	for _, addr := range []string{"cc@example.com", "bcc@example.com"} {
		if err := s.Send(context.Background(), makeSenderParams(addr)); !errors.Is(err, ErrRateLimited) {
			t.Errorf("%s: err = %v, want ErrRateLimited", addr, err)
		}
	}
}

func TestTokenRefreshesAfterInterval(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.GlobalBurst = 100
	cfg.GlobalInterval = time.Minute
	s, now := newTestSender(t, cfg)

	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}

	*now = now.Add(time.Minute)
	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); err != nil {
		t.Errorf("after interval: %v, want nil", err)
	}
}

func TestEmptyRecipientsErrors(t *testing.T) {
	next := &mockSender{}
	s, _ := newTestSender(t, baseConfig(next))

	if err := s.Send(context.Background(), sender.SendParams{
		Subject: "subject", Body: "body",
	}); err == nil {
		t.Fatal("err is nil, want 'must have at least one recipient'")
	}
}

func TestSweepEvictsIdleAddresses(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.AddressBurst = 2
	cfg.GlobalBurst = 100
	cfg.SweepCadenceDuration = time.Minute
	s, now := newTestSender(t, cfg)

	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.addressTokenBucketMap["recipient@example.com"]; !ok {
		t.Fatal("bucket for recipient@example.com not created")
	}

	*now = now.Add(3 * time.Minute)
	if err := s.Send(context.Background(), makeSenderParams("recipient2@example.com")); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.addressTokenBucketMap["recipient@example.com"]; ok {
		t.Error("idle bucket for recipient@example.com was not evicted")
	}
}

func TestSweepRespectsCadence(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.GlobalBurst = 100
	cfg.SweepCadenceDuration = time.Hour
	s, now := newTestSender(t, cfg)

	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); err != nil {
		t.Fatal(err)
	}

	*now = now.Add(time.Minute)
	if err := s.Send(context.Background(), makeSenderParams("recipient@example.com")); err != nil {
		t.Fatal(err)
	}

	if _, ok := s.addressTokenBucketMap["recipient@example.com"]; !ok {
		t.Error("bucket evicted before sweep cadence elapsed")
	}
}

func TestConcurrentSendsRespectGlobalBurst(t *testing.T) {
	next := &mockSender{}
	cfg := baseConfig(next)
	cfg.AddressBurst = 1000
	cfg.GlobalBurst = 10
	s, _ := newTestSender(t, cfg)

	const goroutines = 100
	var wg sync.WaitGroup
	var rateLimited int
	var mu sync.Mutex
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := s.Send(context.Background(), makeSenderParams("user@example.com"))
			if errors.Is(err, ErrRateLimited) {
				mu.Lock()
				rateLimited++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if next.callCount() != 10 {
		t.Errorf("next called %d times, want 10", next.callCount())
	}
	if rateLimited != goroutines-10 {
		t.Errorf("rate limited %d times, want %d", rateLimited, goroutines-10)
	}
}
