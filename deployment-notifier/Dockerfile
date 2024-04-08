FROM golang:1.21-alpine as build

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY main.go .

RUN CGO_ENABLED=0 go build -trimpath -o /notifier

FROM gcr.io/distroless/static-debian12
COPY --from=build /notifier /notifier
EXPOSE 8443
CMD [ "/notifier" ]
