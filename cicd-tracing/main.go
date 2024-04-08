package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/yquansah/cicd-tracing/internal/coordinator"
	"github.com/yquansah/cicd-tracing/internal/handler"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func newResource(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx, resource.WithAttributes(
		semconv.ServiceNameKey.String("deployment"),
		semconv.ServiceVersionKey.String("1.0.0"),
	), resource.WithFromEnv())
}

func run() error {
	if err := godotenv.Load(); err != nil {
		return err
	}

	preamble := "I am a bot."

	githubApiURL := os.Getenv("GITHUB_API_URL")
	githubAppID := os.Getenv("GITHUB_APP_ID")
	privateKeyBase64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_BASE64")
	githubAppWebhookSecret := os.Getenv("GITHUB_APP_WEBHOOK_SECRET")
	zipkinEndpoint := os.Getenv("ZIPKIN_ENDPOINT")

	appID, err := strconv.Atoi(githubAppID)
	if err != nil {
		return err
	}

	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return err
	}

	app := struct {
		IntegrationID int64  `yaml:"integration_id" json:"integrationId"`
		WebhookSecret string `yaml:"webhook_secret" json:"webhookSecret"`
		PrivateKey    string `yaml:"private_key" json:"privateKey"`
	}{
		IntegrationID: int64(appID),
		WebhookSecret: githubAppWebhookSecret,
		PrivateKey:    string(privateKeyBytes),
	}

	// Create a new GitHub App client
	config := githubapp.Config{
		V3APIURL: githubApiURL,
		App:      app,
	}

	cc, err := githubapp.NewDefaultCachingClientCreator(
		config,
		githubapp.WithClientTimeout(3*time.Second),
	)
	if err != nil {
		return err
	}

	mux := chi.NewRouter()

	zipkinExporter, err := zipkin.New(zipkinEndpoint)
	if err != nil {
		return err
	}

	// Tracing registration/configuration
	spanProcessor := trace.NewSimpleSpanProcessor(zipkinExporter)

	rsc, err := newResource(context.Background())
	if err != nil {
		return err
	}

	tracingProvider := trace.NewTracerProvider(trace.WithResource(rsc), trace.WithSampler(trace.AlwaysSample()))
	tracingProvider.RegisterSpanProcessor(spanProcessor)
	deploymentTracer := tracingProvider.Tracer("deployment")

	// Get coordinator
	coordinatorClient := coordinator.NewClient()

	pushHandler := handler.NewPushHandler(cc, preamble, deploymentTracer, coordinatorClient)
	checkSuiteHandler := handler.NewCheckSuiteHandler(cc, preamble, deploymentTracer, coordinatorClient)
	checkRunHandler := handler.NewCheckRunHandler(cc, preamble, deploymentTracer, coordinatorClient)

	webhookHandler := githubapp.NewDefaultEventDispatcher(config, checkSuiteHandler, pushHandler, checkRunHandler)

	mux.Handle(githubapp.DefaultWebhookRoute, webhookHandler)

	mux.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong!"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	fmt.Printf("Running on port %d...\n", 8080)

	return server.ListenAndServe()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
