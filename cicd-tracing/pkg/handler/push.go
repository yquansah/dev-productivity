package handler

import (
	"context"
	"encoding/json"

	"github.com/google/go-github/v60/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/yquansah/cicd-tracing/pkg/coordinator"
	"go.opentelemetry.io/otel/trace"
)

type PushHandler struct {
	githubapp.ClientCreator
	preamble string

	tracer      trace.Tracer
	coordinator *coordinator.Client
}

func NewPushHandler(cc githubapp.ClientCreator, preamble string, tracer trace.Tracer, coordinator *coordinator.Client) *PushHandler {
	return &PushHandler{
		ClientCreator: cc,
		preamble:      preamble,
		tracer:        tracer,
		coordinator:   coordinator,
	}
}

func (p *PushHandler) Handles() []string {
	return []string{"push"}
}

func (p *PushHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event github.PushEvent

	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "failed to parse issue comment event payload")
	}

	commitID := event.HeadCommit.GetID()

	_, span := p.tracer.Start(ctx, commitID)

	err := p.coordinator.Put(commitID, span)
	if err != nil {
		return err
	}

	return nil
}
