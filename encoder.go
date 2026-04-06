package unilog

// Encoder transforms a log event into bytes for transport sinks.
type Encoder interface {
	Name() string
	Encode(Event) ([]byte, error)
	ContentType() string
}
