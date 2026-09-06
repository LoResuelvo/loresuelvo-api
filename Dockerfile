FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git build-base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app ./cmd/api

FROM migrate/migrate:v4.18.3 AS migrate

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/app ./app
COPY --from=migrate /usr/local/bin/migrate /usr/local/bin/migrate
COPY db/migrations /migrations
EXPOSE 8080
CMD ["./app"]
