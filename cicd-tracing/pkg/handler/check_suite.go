package handler

import (
	"context"
	"encoding/json"

	"github.com/google/go-github/v60/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/yquansah/cicd-tracing/pkg/coordinator"
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

func (c *CheckSuiteHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event github.CheckSuiteEvent

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	sha := event.CheckSuite.GetHeadSHA()

	span, err := c.coordinator.Get(sha)
	if err != nil {
		return err
	}
	span.End()

	return nil
}
