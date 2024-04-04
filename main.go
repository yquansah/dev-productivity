package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/github"
	"github.com/slack-go/slack"
	"golang.org/x/oauth2"
	admv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Client struct {
	githubClient     *github.Client
	slackChannelName string
	slackClient      *slack.Client
}

func (c *Client) ping(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("pong!"))
}

func constructFailureResponse(code int32, message string) *metav1.Status {
	return &metav1.Status{
		Code:    code,
		Message: message,
		Status:  "Failure",
	}
}

func (c *Client) mutate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	var review admv1.AdmissionReview
	var deployment appsv1.Deployment

	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := &admv1.AdmissionResponse{
		UID:     review.Request.UID,
		Allowed: true,
	}

	if err := json.Unmarshal(review.Request.Object.Raw, &deployment); err != nil {
		response.Result = constructFailureResponse(http.StatusInternalServerError, err.Error())

		review.Response = response

		if err := json.NewEncoder(w).Encode(review); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	// Get labels from deployment object
	var labels = deployment.GetLabels()

	organizationOwner := labels["app.organization/owner"]
	organizationRepositoryName := labels["app.organization.repository/name"]
	organizationRepositoryCommitHash := labels["app.organization.repository/commit-hash"]

	if organizationOwner == "" {
		response.Result = constructFailureResponse(http.StatusBadRequest, "please provide app.organization/owner label")

		review.Response = response

		if err := json.NewEncoder(w).Encode(review); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if organizationRepositoryName == "" {
		response.Result = constructFailureResponse(http.StatusBadRequest, "please provide app.organization.repository/name label")

		review.Response = response

		if err := json.NewEncoder(w).Encode(review); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if organizationRepositoryCommitHash == "" {
		response.Result = constructFailureResponse(http.StatusBadRequest, "please provide app.organization.repository/commit-hash label")

		review.Response = response

		if err := json.NewEncoder(w).Encode(review); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	commit, resp, err := c.githubClient.Repositories.GetCommit(ctx, organizationOwner, organizationRepositoryName, organizationRepositoryCommitHash)
	if err != nil {
		response.Result = constructFailureResponse(http.StatusInternalServerError, err.Error())

		review.Response = response

		if err := json.NewEncoder(w).Encode(review); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := github.CheckResponse(resp.Response); err != nil {
		response.Result = constructFailureResponse(http.StatusInternalServerError, err.Error())

		review.Response = response

		if err := json.NewEncoder(w).Encode(review); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	conversations, _, err := c.slackClient.GetConversationsContext(ctx, &slack.GetConversationsParameters{})
	if err != nil {
		response.Result = constructFailureResponse(http.StatusInternalServerError, err.Error())

		review.Response = response

		if err := json.NewEncoder(w).Encode(review); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var channelID string

	for _, conversation := range conversations {
		if conversation.Name == c.slackChannelName {
			channelID = conversation.ID
			break
		}
	}

	if channelID == "" {
		response.Result = constructFailureResponse(http.StatusNotFound, fmt.Sprintf("channel %s not found", c.slackChannelName))

		review.Response = response

		if err := json.NewEncoder(w).Encode(review); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	_, _, err = c.slackClient.PostMessage(channelID, slack.MsgOptionText(
		fmt.Sprintf("The deployment: `%s` is rolling out now 🚢 for the repository: `%s`\n\n    - version/commit hash 🆕 : `%s`\n\n    - author ✍🏿 : `%s`\n\n    - committer 🎯 : `%s`",
			deployment.Name,
			organizationRepositoryName,
			organizationRepositoryCommitHash,
			*commit.Author.Login,
			*commit.Committer.Login,
		), true))
	if err != nil {
		response.Warnings = append(response.Warnings, err.Error())
	}

	response.Allowed = true
	response.Result = &metav1.Status{
		Status: "Success",
	}

	review.Response = response

	if err := json.NewEncoder(w).Encode(review); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func getEnvOrReturnError(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("provide %s environment variable", name)
	}

	return value, nil
}

func run() error {
	tlsCertPath, err := getEnvOrReturnError("TLS_CERT_PATH")
	if err != nil {
		return err
	}

	tlsKeyPath, err := getEnvOrReturnError("TLS_KEY_PATH")
	if err != nil {
		return err
	}

	slackApiToken, err := getEnvOrReturnError("SLACK_API_TOKEN")
	if err != nil {
		return err
	}

	slackChannelName, err := getEnvOrReturnError("SLACK_CHANNEL_NAME")
	if err != nil {
		return err
	}

	githubToken := os.Getenv("GITHUB_API_TOKEN")

	certificate, err := tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()

	// initialize slack client
	slackClient := slack.New(slackApiToken)

	// initialize github client
	var httpClient *http.Client = nil

	if githubToken != "" {
		ts := oauth2.StaticTokenSource(

			&oauth2.Token{AccessToken: githubToken},
		)

		httpClient = oauth2.NewClient(context.Background(), ts)
	}

	githubClient := github.NewClient(httpClient)

	s := &Client{
		slackChannelName: slackChannelName,
		slackClient:      slackClient,
		githubClient:     githubClient,
	}

	mux.HandleFunc("/ping", s.ping)
	mux.HandleFunc("/mutate", s.mutate)

	server := &http.Server{
		Addr:         ":8443",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		TLSConfig:    &tls.Config{Certificates: []tls.Certificate{certificate}},
	}

	return server.ListenAndServeTLS("", "")
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
