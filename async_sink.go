package unilog

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

type AsyncSinkOptions struct {
	BufferSize     int
	OverflowPolicy OverflowPolicy
	OnError        func(error)
}

type AsyncSinkStats struct {
	Enqueued uint64
	Dropped  uint64
}

type asyncJob struct {
	ctx   context.Context
	event *Event
	flush chan error
	close chan error
}

type AsyncSink struct {
	next Sink
	opts AsyncSinkOptions

	jobs chan asyncJob
	done chan struct{}

	mu      sync.RWMutex
	closing bool
	closed  bool

	errMu      sync.Mutex
	workerErrs []error

	enqueued atomic.Uint64
	dropped  atomic.Uint64
}

func NewAsyncSink(next Sink, opts AsyncSinkOptions) *AsyncSink {
	if next == nil {
		next = NopSink{}
	}
	if opts.BufferSize <= 0 {
		opts.BufferSize = 256
	}
	if opts.OnError == nil {
		opts.OnError = func(error) {}
	}
	if opts.OverflowPolicy != OverflowBlock && opts.OverflowPolicy != OverflowDropNewest {
		opts.OverflowPolicy = OverflowDropNewest
	}

	a := &AsyncSink{
		next: next,
		opts: opts,
		jobs: make(chan asyncJob, opts.BufferSize),
		done: make(chan struct{}),
	}

	go a.run()
	return a
}

func (a *AsyncSink) Name() string {
	return "async(" + a.opts.OverflowPolicy.String() + ")->" + a.next.Name()
}

func (a *AsyncSink) Write(ctx context.Context, event Event) error {
	job := asyncJob{
		ctx:   contextWithoutCancel(ctx),
		event: ptrEvent(cloneEvent(event)),
	}
	return a.enqueue(ctx, job, false)
}

func (a *AsyncSink) Sync(ctx context.Context) error {
	resp := make(chan error, 1)
	if err := a.enqueue(ctx, asyncJob{ctx: contextWithoutCancel(ctx), flush: resp}, true); err != nil {
		return err
	}
	return waitAsyncResponse(ctx, resp)
}

func (a *AsyncSink) Close(ctx context.Context) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	if a.closing {
		a.mu.Unlock()
		<-a.done
		return nil
	}
	a.closing = true
	a.mu.Unlock()

	resp := make(chan error, 1)
	if err := a.enqueue(ctx, asyncJob{ctx: contextWithoutCancel(ctx), close: resp}, true); err != nil {
		a.mu.Lock()
		a.closing = false
		a.mu.Unlock()
		return err
	}

	err := waitAsyncResponse(ctx, resp)
	<-a.done
	return err
}

func (a *AsyncSink) Stats() AsyncSinkStats {
	return AsyncSinkStats{
		Enqueued: a.enqueued.Load(),
		Dropped:  a.dropped.Load(),
	}
}

func (a *AsyncSink) enqueue(ctx context.Context, job asyncJob, force bool) error {
	a.mu.RLock()
	closing := a.closing
	closed := a.closed
	a.mu.RUnlock()

	isControlJob := job.flush != nil || job.close != nil

	if closed {
		return ErrAsyncSinkClosed
	}
	if closing && !isControlJob {
		return ErrAsyncSinkClosed
	}

	if force || a.opts.OverflowPolicy == OverflowBlock {
		select {
		case a.jobs <- job:
			a.enqueued.Add(1)
			return nil
		case <-ctxDone(ctx):
			return context.Cause(ctx)
		case <-a.done:
			return ErrAsyncSinkClosed
		}
	}

	select {
	case a.jobs <- job:
		a.enqueued.Add(1)
		return nil
	default:
		a.dropped.Add(1)
		a.opts.OnError(ErrAsyncBufferFull)
		return nil
	}
}

func (a *AsyncSink) run() {
	defer close(a.done)
	defer func() {
		a.mu.Lock()
		a.closed = true
		a.closing = false
		a.mu.Unlock()
	}()

	for {
		job := <-a.jobs
		switch {
		case job.event != nil:
			if err := a.next.Write(job.ctx, *job.event); err != nil {
				a.recordWorkerError(err)
			}
		case job.flush != nil:
			err := joinErrors(a.drainWorkerErrors(), a.next.Sync(job.ctx))
			job.flush <- err
		case job.close != nil:
			err := joinErrors(a.drainWorkerErrors(), a.next.Sync(job.ctx), a.next.Close(job.ctx))
			job.close <- err
			return
		}
	}
}

func (a *AsyncSink) recordWorkerError(err error) {
	if err == nil {
		return
	}

	a.errMu.Lock()
	a.workerErrs = append(a.workerErrs, err)
	a.errMu.Unlock()

	a.opts.OnError(err)
}

func (a *AsyncSink) drainWorkerErrors() error {
	a.errMu.Lock()
	defer a.errMu.Unlock()

	if len(a.workerErrs) == 0 {
		return nil
	}

	err := errors.Join(a.workerErrs...)
	a.workerErrs = nil
	return err
}

func waitAsyncResponse(ctx context.Context, resp <-chan error) error {
	select {
	case err := <-resp:
		return err
	case <-ctxDone(ctx):
		return context.Cause(ctx)
	}
}

func ctxDone(ctx context.Context) <-chan struct{} {
	if ctx == nil {
		return nil
	}
	return ctx.Done()
}

func contextWithoutCancel(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func cloneEvent(ev Event) Event {
	out := ev
	out.Fields = cloneFields(ev.Fields)
	if ev.Caller != nil {
		c := *ev.Caller
		out.Caller = &c
	}
	return out
}

func ptrEvent(ev Event) *Event {
	return &ev
}
