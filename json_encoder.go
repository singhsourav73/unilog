package unilog

import "encoding/json"

type JSONEncoder struct{}

func NewJSONEncoder() *JSONEncoder { return &JSONEncoder{} }

func (e *JSONEncoder) Name() string { return "json" }

func (e *JSONEncoder) ContentType() string { return "application/json" }

func (e *JSONEncoder) Encode(event Event) ([]byte, error) {
	return json.Marshal(EventPayload(event))
}
