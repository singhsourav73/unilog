package unilog

import "errors"

var (
	ErrDropEvent       = errors.New("unilog: drop event")
	ErrAsyncBufferFull = errors.New("unilog: async sink buffer full")
	ErrAsyncSinkClosed = errors.New("unilog: async sink closed")
	ErrCircuitOpen     = errors.New("unilog: circuit breaker is open")
)

func IsDropEvent(err error) bool {
	return errors.Is(err, ErrDropEvent)
}
