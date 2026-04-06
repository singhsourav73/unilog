package unilog

import "context"

type LevelFilterSink struct {
	min  Level
	next Sink
}

func NewLevelFilterSink(min Level, next Sink) *LevelFilterSink {
	if next == nil {
		next = NopSink{}
	}

	return &LevelFilterSink{min: min, next: next}
}

func (s *LevelFilterSink) Name() string {
	return "level_filter(" + s.min.String() + ")->" + s.next.Name()
}

func (s *LevelFilterSink) Write(ctx context.Context, event Event) error {
	if !event.Level.Valid() || event.Level < s.min {
		return nil
	}
	return s.next.Write(ctx, event)
}

func (s *LevelFilterSink) Sync(ctx context.Context) error {
	if syncer, ok := s.next.(interface{ Sync(context.Context) error }); ok {
		return syncer.Sync(ctx)
	}
	return nil
}

func (s *LevelFilterSink) Close(ctx context.Context) error {
	if closer, ok := s.next.(interface{ Close(context.Context) error }); ok {
		return closer.Close(ctx)
	}
	return nil
}
