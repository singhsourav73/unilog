package unilog

import "context"

type NopSink struct{}

func (NopSink) Name() string { return "nop" }

func (NopSink) Write(context.Context, Event) error { return nil }

func (NopSink) Sync(context.Context) error { return nil }

func (NopSink) Close(context.Context) error { return nil }
