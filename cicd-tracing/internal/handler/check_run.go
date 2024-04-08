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
	return []string{"check_run"}
}

func getCheckRunName(sha, checkRunName string) string {
	return fmt.Sprintf("%s_check_run_%s", sha, checkRunName)
}

func (c *CheckRunHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event github.CheckRunEvent

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	sha := event.GetCheckRun().GetHeadSHA()
	action := event.GetAction()
	checkRunName := event.GetCheckRun().GetName()
	checkRunKey := getCheckRunName(sha, checkRunName)

	if action == "created" {
		findParent := func(key string) trace.Span {
			for {
				span, _ := c.coordinator.Get(key)
				if span != nil {
					return span
				}
				time.Sleep(100 * time.Millisecond)
			}
		}

		checkSuiteSpan := findParent(getCheckSuiteKey(sha))
		_, checkRunSpan := c.tracer.Start(trace.ContextWithSpan(ctx, checkSuiteSpan), checkRunKey)

		err := c.coordinator.Put(checkRunKey, checkRunSpan)
		if err != nil {
			return err
		}
	}

	if action == "completed" {
		span, err := c.coordinator.Get(checkRunKey)
		if err != nil {
			return err
		}
		span.End()
	}

	return nil
}
