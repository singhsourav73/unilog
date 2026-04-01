package unilog

import "errors"

var (
	ErrDropEvent       = errors.New("unilog: drop event")
	ErrAsyncBufferFull = errors.New("unilog: async sink buffer full")
	ErrAsyncSinkClosed = errors.New("unilog: async sink closed")
)

func IsDropEvent(err error) bool {
	return errors.Is(err, ErrDropEvent)
}
