package unilog

import (
	"context"
	"sync"
	"time"
)

type BatchWriter interface {
	WriteBatch(ctx context.Context, events []Event) error
}

type BatchSinkOptions struct {
	MaxBatchSize   int
	FlushInterval  time.Duration
	MaxQueueSize   int
	OverflowPolicy OverflowPolicy
	OnError        func(error)
}

type BatchSinkStats struct {
	Enqueued    uint64
	Flushed     uint64
	Dropped     uint64
	Batches     uint64
	WriteErrors uint64
	Flushes     uint64
	Closes      uint64
}

type batchJob struct {
	ctx   context.Context
	event *Event
	flush chan error
	close chan error
}

type BatchSink struct {
	next Sink
	opts BatchSinkOptions

	jobs chan batchJob
	done chan struct{}

	mu      sync.RWMutex
	closing bool
	closed  bool

	statsMu sync.Mutex
	stats   BatchSinkStats
}

func NewBatchSink(next Sink, opts BatchSinkOptions) *BatchSink {
	if next == nil {
		next = NopSink{}
	}
	if opts.MaxBatchSize <= 0 {
		opts.MaxBatchSize = 100
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = time.Second
	}
	if opts.MaxQueueSize <= 0 {
		opts.MaxQueueSize = opts.MaxBatchSize * 4
	}
	if opts.OverflowPolicy != OverflowBlock && opts.OverflowPolicy != OverflowDropNewest {
		opts.OverflowPolicy = OverflowDropNewest
	}
	if opts.OnError == nil {
		opts.OnError = func(error) {}
	}

	b := &BatchSink{
		next: next,
		opts: opts,
		jobs: make(chan batchJob, opts.MaxQueueSize),
		done: make(chan struct{}),
	}

	go b.run()
	return b
}

func (b *BatchSink) Name() string {
	return "batch(" + b.opts.OverflowPolicy.String() + ")->" + b.next.Name()
}

func (b *BatchSink) Write(ctx context.Context, event Event) error {
	job := batchJob{
		ctx:   contextWithoutCancel(ctx),
		event: ptrEvent(cloneEvent(event)),
	}
	return b.enqueue(ctx, job, false)
}

func (b *BatchSink) Sync(ctx context.Context) error {
	resp := make(chan error, 1)
	if err := b.enqueue(ctx, batchJob{
		ctx:   contextWithoutCancel(ctx),
		flush: resp,
	}, true); err != nil {
		return err
	}
	return waitAsyncResponse(ctx, resp)
}

func (b *BatchSink) Close(ctx context.Context) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	if b.closing {
		b.mu.Unlock()
		select {
		case <-b.done:
			return nil
		case <-ctxDone(ctx):
			return context.Cause(ctx)
		}
	}
	b.closing = true
	b.mu.Unlock()

	resp := make(chan error, 1)
	if err := b.enqueue(ctx, batchJob{
		ctx:   contextWithoutCancel(ctx),
		close: resp,
	}, true); err != nil {
		b.mu.Lock()
		b.closing = false
		b.mu.Unlock()
		return err
	}

	err := waitAsyncResponse(ctx, resp)
	<-b.done
	return err
}

func (b *BatchSink) Stats() BatchSinkStats {
	b.statsMu.Lock()
	defer b.statsMu.Unlock()
	return b.stats
}

func (b *BatchSink) enqueue(ctx context.Context, job batchJob, force bool) error {
	b.mu.RLock()
	closing := b.closing
	closed := b.closed
	b.mu.RUnlock()

	isControlJob := job.flush != nil || job.close != nil

	if closed {
		return ErrAsyncSinkClosed
	}
	if closing && !isControlJob {
		return ErrAsyncSinkClosed
	}

	if force || b.opts.OverflowPolicy == OverflowBlock {
		select {
		case b.jobs <- job:
			b.incEnqueued()
			return nil
		case <-ctxDone(ctx):
			return context.Cause(ctx)
		case <-b.done:
			return ErrAsyncSinkClosed
		}
	}

	select {
	case b.jobs <- job:
		b.incEnqueued()
		return nil
	default:
		b.incDropped()
		b.opts.OnError(ErrAsyncBufferFull)
		return nil
	}
}

func (b *BatchSink) run() {
	defer close(b.done)
	defer func() {
		b.mu.Lock()
		b.closed = true
		b.closing = false
		b.mu.Unlock()
	}()

	ticker := time.NewTicker(b.opts.FlushInterval)
	defer ticker.Stop()

	batch := make([]Event, 0, b.opts.MaxBatchSize)

	flushBatch := func(ctx context.Context) error {
		if len(batch) == 0 {
			return nil
		}

		toFlush := make([]Event, len(batch))
		copy(toFlush, batch)
		batch = batch[:0]

		b.incBatches()

		var err error
		if bw, ok := b.next.(BatchWriter); ok {
			err = bw.WriteBatch(ctx, toFlush)
			if err == nil {
				b.incFlushed(uint64(len(toFlush)))
			} else {
				b.incWriteErrors()
				b.opts.OnError(err)
			}
		} else {
			for _, ev := range toFlush {
				if e := b.next.Write(ctx, ev); e != nil {
					err = joinErrors(err, e)
				}
			}
		}

		if err == nil {
			b.incFlushed(uint64(len(toFlush)))
		} else {
			b.opts.OnError(err)
		}
		return err
	}

	for {
		select {
		case <-ticker.C:
			_ = flushBatch(context.Background())

		case job := <-b.jobs:
			switch {
			case job.event != nil:
				batch = append(batch, *job.event)
				if len(batch) >= b.opts.MaxBatchSize {
					_ = flushBatch(job.ctx)
				}

			case job.flush != nil:
				b.incFlushes()
				err := joinErrors(flushBatch(job.ctx), b.next.Sync(job.ctx))
				job.flush <- err

			case job.close != nil:
				b.incCloses()
				err := joinErrors(flushBatch(job.ctx), b.next.Sync(job.ctx), b.next.Close(job.ctx))
				job.close <- err
				return
			}
		}
	}
}

func (b *BatchSink) incEnqueued() {
	b.statsMu.Lock()
	b.stats.Enqueued++
	b.statsMu.Unlock()
}

func (b *BatchSink) incDropped() {
	b.statsMu.Lock()
	b.stats.Dropped++
	b.statsMu.Unlock()
}

func (b *BatchSink) incFlushed(n uint64) {
	b.statsMu.Lock()
	b.stats.Flushed += n
	b.statsMu.Unlock()
}

func (b *BatchSink) incWriteErrors() {
	b.statsMu.Lock()
	b.stats.WriteErrors++
	b.statsMu.Unlock()
}

func (b *BatchSink) incFlushes() {
	b.statsMu.Lock()
	b.stats.Flushes++
	b.statsMu.Unlock()
}

func (b *BatchSink) incCloses() {
	b.statsMu.Lock()
	b.stats.Closes++
	b.statsMu.Unlock()
}

func (b *BatchSink) incBatches() {
	b.statsMu.Lock()
	b.stats.Batches++
	b.statsMu.Unlock()
}
