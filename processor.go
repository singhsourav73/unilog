package unilog

import "context"

type Processor interface {
	Name() string
	Process(ctx context.Context, event *Event) error
}
