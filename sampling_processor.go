package unilog

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

type SamplingOptions struct {
	DebugEvery uint64
	InfoEvery  uint64
	WarnEvery  uint64
}

type SamplingProcessor struct {
	debugEvery uint64
	infoEvery  uint64
	warnEvery  uint64

	counters sync.Map // map[string]*atomic.Uint64
}

func NewSamplingProcessor(opts SamplingOptions) *SamplingProcessor {
	return &SamplingProcessor{
		debugEvery: normailzeEvery(opts.DebugEvery),
		infoEvery:  normailzeEvery(opts.InfoEvery),
		warnEvery:  normailzeEvery(opts.WarnEvery),
	}
}

func (p *SamplingProcessor) Name() string {
	return "sampling"
}

func (p *SamplingProcessor) Process(_ context.Context, event *Event) error {
	if event == nil {
		return nil
	}

	every := p.everyFor(event.Level)
	if every <= 1 || event.Level >= ErrorLevel {
		return nil
	}

	key := p.counterKey(event)
	n := p.next(key)

	if (n-1)%every != 0 {
		return ErrDropEvent
	}

	return nil
}

func (p *SamplingProcessor) everyFor(level Level) uint64 {
	switch level {
	case DebugLevel:
		return p.debugEvery
	case InfoLevel:
		return p.infoEvery
	case WarnLevel:
		return p.warnEvery
	default:
		return 1
	}
}

func (p *SamplingProcessor) counterKey(event *Event) string {
	return fmt.Sprintf("%d|%s|%s", event.Level, event.LoggerName, event.Message)
}

func (p *SamplingProcessor) next(key string) uint64 {
	actual, _ := p.counters.LoadOrStore(key, &atomic.Uint64{})
	counter := actual.(*atomic.Uint64)
	return counter.Add(1)
}

func normailzeEvery(v uint64) uint64 {
	if v == 0 {
		return 1
	}
	return v
}
