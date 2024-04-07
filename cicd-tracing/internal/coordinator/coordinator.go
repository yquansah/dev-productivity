package coordinator

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

type Client struct {
	spanCollector map[string]trace.Span
	mtx           sync.RWMutex
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

	c.mtx.Lock()
	c.spanCollector[key] = span
	c.mtx.Unlock()

	return nil
}

func (c *Client) Get(key string) (trace.Span, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	return c.get(key)
}

func (c *Client) get(key string) (trace.Span, error) {
	span, ok := c.spanCollector[key]
	if !ok {
		return nil, fmt.Errorf("key: %s does not exist in collection", key)
	}

	return span, nil
}
