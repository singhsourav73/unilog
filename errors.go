package unilog

import "errors"

var ErrDropEvent = errors.New("unilog: drop event")

func IsDropEvent(err error) bool {
	return errors.Is(err, ErrDropEvent)
}
