package unilog

import (
	"context"
	"time"
)

type Handler func(ctx context.Context, event Event) error

type Interceptor func(next Handler) Handler

func ChainInterceptors(final Handler, interceptors ...Interceptor) Handler {
	h := final
	for i := len(interceptors) - 1; i >= 0; i-- {
		if interceptors[i] != nil {
			h = interceptors[i](h)
		}
	}
	return h
}

type TimingObservation struct {
	Level    Level
	Message  string
	Duration time.Duration
	Err      error
}

func TimingInterceptor(observe func(ctx context.Context, obs TimingObservation)) Interceptor {
	if observe == nil {
		observe = func(ctx context.Context, obs TimingObservation) {}
	}

	return func(next Handler) Handler {
		return func(ctx context.Context, event Event) error {
			start := time.Now()
			err := next(ctx, event)
			observe(ctx, TimingObservation{
				Level:    event.Level,
				Message:  event.Message,
				Duration: time.Since(start),
				Err:      err,
			})
			return err
		}
	}
}

func RecoverInterceptor(onPanic func(ctx context.Context, recovered any)) Interceptor {
	if onPanic == nil {
		onPanic = func(context.Context, any) {}
	}

	return func(next Handler) Handler {
		return func(ctx context.Context, event Event) (err error) {
			defer func() {
				if r := recover(); r != nil {
					onPanic(ctx, r)
					err = nil
				}
			}()
			return next(ctx, event)
		}
	}
}
