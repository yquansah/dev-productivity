package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/yquansah/cicd-tracing/internal/coordinator"
	"go.opentelemetry.io/otel/trace"
)

type CheckSuiteHandler struct {
	githubapp.ClientCreator
	preamble string

	tracer      trace.Tracer
	coordinator *coordinator.Client
}

func NewCheckSuiteHandler(cc githubapp.ClientCreator, preamble string, tracer trace.Tracer, coordinator *coordinator.Client) *CheckSuiteHandler {
	return &CheckSuiteHandler{
		ClientCreator: cc,
		preamble:      preamble,
		tracer:        tracer,
		coordinator:   coordinator,
	}
}

func (c *CheckSuiteHandler) Handles() []string {
	return []string{"check_suite"}
}

func getCheckSuiteKey(sha string) string {
	return fmt.Sprintf("%s_check_suite", sha)
}

func (c *CheckSuiteHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event github.CheckSuiteEvent

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	sha := event.GetCheckSuite().GetHeadSHA()
	action := event.GetAction()
	spanKeyName := getCheckSuiteKey(sha)

	if action == "requested" {
		findParent := func(key string) trace.Span {
			for {
				span, _ := c.coordinator.Get(key)
				if span != nil {
					return span
				}
				time.Sleep(100 * time.Millisecond)
			}
		}

		span := findParent(sha)
		_, checkSuiteSpan := c.tracer.Start(trace.ContextWithSpan(ctx, span), spanKeyName)

		err := c.coordinator.Put(spanKeyName, checkSuiteSpan)
		if err != nil {
			return err
		}
	}

	if action == "completed" {
		checkSuiteSpan, err := c.coordinator.Get(spanKeyName)
		if err != nil {
			return err
		}

		checkSuiteSpan.End()

		pushSpan, err := c.coordinator.Get(sha)
		if err != nil {
			return err
		}

		pushSpan.End()
	}

	return nil
}
