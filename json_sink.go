package unilog

import "io"

type JSONSink = WriterSink

func NewJSONSink(w io.Writer) *JSONSink {
	s, err := NewWriterSink(w, NewJSONEncoder())
	if err != nil {
		panic(err)
	}
	return s
}
