package coordinator

import (
	"fmt"

	"go.opentelemetry.io/otel/trace"
)

type Client struct {
	spanCollector map[string]trace.Span
}

func NewClient() *Client {
	return &Client{
		spanCollector: make(map[string]trace.Span),
	}
}

func (c *Client) Put(key string, span trace.Span) error {
	_, ok := c.spanCollector[key]
	if ok {
		return fmt.Errorf("key: %s already exists in collection", key)
	}

	c.spanCollector[key] = span

	return nil
}

func (c *Client) Get(key string) (trace.Span, error) {
	span, ok := c.spanCollector[key]
	if !ok {
		return nil, fmt.Errorf("key: %s does not exist in collection", key)
	}

	return span, nil
}
