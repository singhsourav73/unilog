package unilog

type Encoder interface {
	Name() string
	Encode(Event) ([]byte, error)
	ContentType() string
}
