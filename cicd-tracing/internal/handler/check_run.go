package handler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/yquansah/cicd-tracing/internal/coordinator"
	"go.opentelemetry.io/otel/trace"
)

type CheckRunHandler struct {
	githubapp.ClientCreator
	preamble string

	tracer      trace.Tracer
	coordinator *coordinator.Client
}

func NewCheckRunHandler(cc githubapp.ClientCreator, preamble string, tracer trace.Tracer, coordinator *coordinator.Client) *CheckRunHandler {
	return &CheckRunHandler{
		ClientCreator: cc,
		preamble:      preamble,
		tracer:        tracer,
		coordinator:   coordinator,
	}
}

func (c *CheckRunHandler) Handles() []string {
	return []string{"check_suite"}
}

func (c *CheckRunHandler) findParent(key string) trace.Span {
	for {
		span, _ := c.coordinator.Get(key)
		if span != nil {
			return span
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (c *CheckRunHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event github.CheckRun

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	_ = event.GetCheckSuite().GetHeadSHA()

	// action := event.GetAction()
	//
	// if action == "requested" {
	// 	span := c.findParent(sha)
	//
	// 	spanKeyName := getCheckSuiteKey(sha)
	//
	// 	_, checkSuiteSpan := c.tracer.Start(trace.ContextWithSpan(context.Background(), span), spanKeyName)
	//
	// 	err := c.coordinator.Put(spanKeyName, checkSuiteSpan)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	//
	// if action == "completed" {
	// 	spanKeyRequestedName := getCheckSuiteKey(sha)
	//
	// 	checkSuiteSpan, err := c.coordinator.Get(spanKeyRequestedName)
	// 	if err != nil {
	// 		return err
	// 	}
	//
	// 	checkSuiteSpan.End()
	//
	// 	pushSpan, err := c.coordinator.Get(sha)
	// 	if err != nil {
	// 		return err
	// 	}
	//
	// 	pushSpan.End()
	// }

	return nil
}
