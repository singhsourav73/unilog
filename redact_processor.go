package unilog

import (
	"context"
	"strings"
)

type RedactionOptions struct {
	Keys            []string
	Mask            string
	CaseInsensitive bool
	RedactError     bool
}

type RedactionProcessor struct {
	mask            string
	keys            map[string]struct{}
	caseInsensitive bool
	redactError     bool
}

func NewRedactionProcessor(opts RedactionOptions) *RedactionProcessor {
	mask := opts.Mask
	if mask == "" {
		mask = "***REDACTED***"
	}

	keys := make(map[string]struct{}, len(opts.Keys))
	for _, k := range opts.Keys {
		if opts.CaseInsensitive {
			k = strings.ToLower(strings.TrimSpace(k))
		} else {
			k = strings.TrimSpace(k)
		}
		if k != "" {
			keys[k] = struct{}{}
		}
	}

	return &RedactionProcessor{
		mask:            mask,
		keys:            keys,
		caseInsensitive: opts.CaseInsensitive,
		redactError:     opts.RedactError,
	}
}

func (p *RedactionProcessor) Name() string {
	return "redaction"
}

func (p *RedactionProcessor) Process(_ context.Context, event *Event) error {
	if event == nil {
		return nil
	}

	for i := range event.Fields {
		event.Fields[i].Value = p.redactValue(event.Fields[i].Key, event.Fields[i].Value)
	}

	if p.redactError && event.Err != nil && p.shouldRedact("error") {
		event.Err = maskedError(p.mask)
	}

	return nil
}

func (p *RedactionProcessor) shouldRedact(key string) bool {
	if p.caseInsensitive {
		key = strings.ToLower(strings.TrimSpace(key))
	} else {
		strings.TrimSpace(key)
	}
	_, ok := p.keys[key]
	return ok
}

func (p *RedactionProcessor) redactValue(key string, value any) any {
	if p.shouldRedact(key) {
		return p.mask
	}

	switch x := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = p.redactValue(k, v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = p.redactNested(v)
		}
		return out
	case []Field:
		out := make([]Field, len(x))
		for i, f := range x {
			out[i] = Field{Key: f.Key, Value: p.redactValue(f.Key, f.Value)}
		}
		return out
	default:
		return value
	}
}

func (p *RedactionProcessor) redactNested(value any) any {
	switch x := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = p.redactValue(k, v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = p.redactNested(v)
		}
		return out
	case []Field:
		out := make([]Field, len(x))
		for i, f := range x {
			out[i] = Field{Key: f.Key, Value: p.redactValue(f.Key, f.Value)}
		}
		return out
	default:
		return value
	}
}

type maskedError string

func (e maskedError) Error() string {
	return string(e)
}
