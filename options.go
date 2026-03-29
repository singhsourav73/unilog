package unilog

import "context"

type InternalErrorHandler func(ctx context.Context, err error)

type Options struct {
	Service     string
	Environment string
	Level       Level
	AddCaller   bool
	AddStack    bool

	// called when a processor or sink returns an internal error.
	// Logging methods themselves do not return errors.
	OnInternalError InternalErrorHandler
}

func (o Options) withDefaults() Options {
	if !o.Level.Valid() {
		o.Level = InfoLevel
	}
	if o.OnInternalError == nil {
		o.OnInternalError = func(context.Context, error) {}
	}
	return o
}
