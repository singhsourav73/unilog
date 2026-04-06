package unilog

import (
	"fmt"
	"io"
	"strings"
)

type TextSinkOptions = TextEncoderOptions

type TextSink = WriterSink

func NewTextSink(w io.Writer, opts TextSinkOptions) *TextSink {
	s, err := NewWriterSink(w, NewTextEncoder(opts))
	if err != nil {
		panic(err)
	}
	return s
}

func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\r\n=\"") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
