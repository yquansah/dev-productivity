# K8s Notifier

## Motivation

There are many situations where developers ask themselves the question: **Is my change deployed?**

The answer to this question differs from organization to organization:
- Go check the service logs
- Go check the CI/CD pipeline status

Or even worse, you find out once it has broken something in an environment.

As developers, we are busy day to day with writing code, doing code reviews, answering slack messages, that in reality the best way to know about things that are happening are push based, not pull based.

This repository provides a way to alert an engineer that their change is shipping/deploying by using [Kubernetes Dynamic Admission Control](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/).

## Details

### Kubernetes

Kubernetes already recommends that operation teams provide a [recommended set of labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/) to their deployment manifests. This means that if you are doing this already in an automated way, you are somehow connecting your CI to your CD (or your CD runs along your CI), and passing metadata between the two entities.

If this is already happening, you can pass the set of labels that is expected for validation to pass for this service:

- `app.organization/owner`
- `app.organization.repository/name`
- `app.organization.repository/commit-hash`

Without these labels, the validating webhook will fail.

In addition, the Kubernetes API server communicates over HTTPS to the validating webhook. There are details on how to set this up [here](https://caffeinecoding.com/create-self-signed-certificate-https/). The service itself expects the following environment variables:

- `TLS_CERT_PATH`: The path to your tls cert on disk
- `TLS_KEY_PATH`: The path to your tls key on disk

The nice way to do this is to create a Kubernetes secret with the `kubernetes.io/tls` type, look [here](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets) for an example.


### Github

The service will consult the GitHub API (you can provide an access token if your GH repo is private) for commit details based on the commit hash, repository name, and repository owner. From this API call, we can get [a load of information about the commit](https://docs.github.com/en/rest/commits/commits?apiVersion=2022-11-28#get-a-commit).

The values for the repository name, owner, and commit hash all come from the labels as described above.

### Slack

The service expects a slack API bot or user token and channel name for communicating with your slack workspace. These are required fields. The necessary environment variables are:

- `SLACK_CHANNEL_NAME`: The name of the slack channel you want to send alerts to
- `SLACK_API_TOKEN`: The API token for authenticating the slack client in the validating webhook service

In order for you to get one of these tokens, you need to create a [Slack App](https://api.slack.com/start/quickstart), and create a token with the requested scopes:

- `channels:read`
- `chat:write`
