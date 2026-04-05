package unilog

import (
	"context"
	"errors"
	"sync"
	"time"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type CircuitBreakerOptions struct {
	FailureThreshold int
	OpenTimeout      time.Duration
	OnStateChange    func(from, to CircuitState)
	Now              func() time.Time
}

type CircuitBreakerSink struct {
	next Sink
	opts CircuitBreakerOptions

	mu                  sync.Mutex
	state               CircuitState
	consecutiveFailures int
	openUntil           time.Time
}

func NewCircuitBreakerSink(next Sink, opts CircuitBreakerOptions) *CircuitBreakerSink {
	if next == nil {
		next = NopSink{}
	}
	if opts.FailureThreshold <= 0 {
		opts.FailureThreshold = 3
	}
	if opts.OpenTimeout <= 0 {
		opts.OpenTimeout = 5 * time.Second
	}
	if opts.OnStateChange == nil {
		opts.OnStateChange = func(from, to CircuitState) {}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	return &CircuitBreakerSink{
		next:  next,
		opts:  opts,
		state: CircuitClosed,
	}
}

func (s *CircuitBreakerSink) Name() string {
	return "circuit_breaker->" + s.next.Name()
}

func (s *CircuitBreakerSink) Write(ctx context.Context, event Event) error {
	if err := s.beforeWrite(); err != nil {
		return err
	}

	err := s.next.Write(ctx, event)
	if err != nil {
		s.onFailure()
		return err
	}

	s.onSuccess()
	return nil
}

func (s *CircuitBreakerSink) Sync(ctx context.Context) error {
	return s.next.Sync(ctx)
}

func (s *CircuitBreakerSink) Close(ctx context.Context) error {
	return s.next.Close(ctx)
}

func (s *CircuitBreakerSink) State() CircuitState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *CircuitBreakerSink) beforeWrite() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.opts.Now()

	switch s.state {
	case CircuitOpen:
		if now.Before(s.openUntil) {
			return ErrCircuitOpen
		}
		s.transitionLocked(CircuitHalfOpen)
		return nil

	case CircuitHalfOpen, CircuitClosed:
		return nil

	default:
		return nil
	}
}

func (s *CircuitBreakerSink) onSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.consecutiveFailures = 0
	if s.state != CircuitClosed {
		s.transitionLocked(CircuitClosed)
	}
}

func (s *CircuitBreakerSink) onFailure() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.opts.Now()

	switch s.state {
	case CircuitHalfOpen:
		s.consecutiveFailures = s.opts.FailureThreshold
		s.openUntil = now.Add(s.opts.OpenTimeout)
		s.transitionLocked(CircuitOpen)
		return

	case CircuitClosed:
		s.consecutiveFailures++
		if s.consecutiveFailures >= s.opts.FailureThreshold {
			s.openUntil = now.Add(s.opts.OpenTimeout)
			s.transitionLocked(CircuitOpen)
		}
		return

	case CircuitOpen:
		s.openUntil = now.Add(s.opts.OpenTimeout)
		return
	}
}

func (s *CircuitBreakerSink) transitionLocked(to CircuitState) {
	from := s.state
	if from == to {
		return
	}
	s.state = to
	s.opts.OnStateChange(from, to)
}

func IsCircuitOpen(err error) bool {
	return errors.Is(err, ErrCircuitOpen)
}
