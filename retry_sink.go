package unilog

import (
	"context"
	"fmt"
	"time"
)

type RetrySinkOptions struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64

	Retryable func(error) bool
	Sleep     func(time.Duration)
}

type RetrySink struct {
	next Sink
	opts RetrySinkOptions
}

func NewRetrySink(next Sink, opts RetrySinkOptions) *RetrySink {
	if next == nil {
		next = NopSink{}
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.InitialBackoff <= 0 {
		opts.InitialBackoff = 50 * time.Millisecond
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 1 * time.Second
	}
	if opts.Multiplier <= 1 {
		opts.Multiplier = 2
	}
	if opts.Retryable == nil {
		opts.Retryable = func(error) bool { return true }
	}
	if opts.Sleep == nil {
		opts.Sleep = time.Sleep
	}

	return &RetrySink{
		next: next,
		opts: opts,
	}
}

func (s *RetrySink) Name() string {
	return "retry->" + s.next.Name()
}

func (s *RetrySink) Write(ctx context.Context, event Event) error {
	return s.do(ctx, "write", func(ctx context.Context) error {
		return s.next.Write(ctx, event)
	})
}

func (s *RetrySink) Sync(ctx context.Context) error {
	return s.do(ctx, "sync", s.next.Sync)
}

func (s *RetrySink) Close(ctx context.Context) error {
	return s.do(ctx, "close", s.next.Close)
}

func (s *RetrySink) do(ctx context.Context, op string, fn func(context.Context) error) error {
	backoff := s.opts.InitialBackoff
	var lastErr error

	for attempts := 1; attempts <= s.opts.MaxAttempts; attempts++ {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err

		if !s.opts.Retryable(err) || attempts == s.opts.MaxAttempts {
			break
		}

		if err := sleepWithContext(ctx, backoff, s.opts.Sleep); err != nil {
			return err
		}

		backoff = nextBackoff(backoff, s.opts.MaxBackoff, s.opts.Multiplier)
	}

	return fmt.Errorf("unilog: %s failed after %d attempt(s): %w", op, s.opts.MaxAttempts, lastErr)
}

func nextBackoff(current, max time.Duration, multiplier float64) time.Duration {
	next := time.Duration(float64(current) * multiplier)
	if next > max {
		return max
	}
	return next
}

func sleepWithContext(ctx context.Context, d time.Duration, sleep func(time.Duration)) error {
	if ctx == nil {
		sleep(d)
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
