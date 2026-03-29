package unilog

import "context"

type Sink interface {
	Name() string
	Write(ctx context.Context, event Event) error
	Sync(ctx context.Context) error
	Close(ctx context.Context) error
}
