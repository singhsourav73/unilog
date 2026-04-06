package unilog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPSinkOptions struct {
	URL     string
	Method  string
	Headers map[string]string
	Timeout time.Duration
	Client  *http.Client
	Encoder Encoder
}

type HTTPSink struct {
	url     string
	method  string
	headers map[string]string
	client  *http.Client
	encoder Encoder
}

func NewHTTPSink(opts HTTPSinkOptions) (*HTTPSink, error) {
	url := strings.TrimSpace(opts.URL)
	if url == "" {
		return nil, fmt.Errorf("unilog: http sink url is empty")
	}

	method := strings.ToUpper(strings.TrimSpace(opts.Method))
	if method == "" {
		method = http.MethodPost
	}

	client := opts.Client
	if client == nil {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}

	headers := make(map[string]string, len(opts.Headers))
	for k, v := range opts.Headers {
		headers[k] = v
	}

	encoder := opts.Encoder
	if encoder == nil {
		encoder = NewJSONEncoder()
	}

	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = encoder.ContentType()
	}

	return &HTTPSink{
		url:     url,
		method:  method,
		headers: headers,
		client:  client,
		encoder: encoder,
	}, nil
}

func (s *HTTPSink) Name() string {
	return "http(" + s.encoder.Name() + ")"
}

func (s *HTTPSink) Write(ctx context.Context, event Event) error {
	body, err := s.encoder.Encode(event)
	if err != nil {
		return fmt.Errorf("unilog: encode event payload: %w", err)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, s.method, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("unilog: build http request: %w", err)
	}

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("unilog: http sink request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("unilog: http sink unexpected status %d, %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	return nil
}

func (s *HTTPSink) Sync(context.Context) error  { return nil }
func (s *HTTPSink) Close(context.Context) error { return nil }
