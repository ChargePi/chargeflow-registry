FROM golang:1.26.0 AS build
WORKDIR /app/src

ENV GOCACHE=/root/.cache/go-build
ENV GOMODCACHE=/root/.cache/go-build
ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOPRIVATE=github.com/ChargePi/*

# Leverage build cache by copying go.mod and go.sum first
COPY go.mod go.sum ./

RUN apt-get update && apt-get install -y --no-install-recommends openssh-client && rm -rf /var/lib/apt/lists/*
RUN mkdir -p -m 0700 /root/.ssh && ssh-keyscan github.com >> /root/.ssh/known_hosts

RUN --mount=type=ssh,required=true \
    --mount=type=cache,target="/root/.cache/go-build" \
    git config --global url."git@github.com:".insteadOf "https://github.com/" && \
    go mod download && go mod verify

# Copy the rest of the source code
COPY . .

RUN --mount=type=cache,target="/root/.cache/go-build" go build -o ../chargeflow-registry ./cmd/app
RUN --mount=type=cache,target="/root/.cache/go-build" go build -o ../migrate ./cmd/migrate

# Debug stage using Delve
FROM build AS debug

COPY --from=build /app/src /service/src

WORKDIR /service/src

# Install Delve
RUN go install github.com/go-delve/delve/cmd/dlv@latest
# Build the application with debugging flags
RUN go build -gcflags="all=-N -l" -o ./chargeflow-registry ./cmd/app

CMD ["/go/bin/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "./chargeflow-registry"]

FROM alpine:3.21 AS app

RUN apk add --no-cache curl ca-certificates wget

# Install grpc_health_probe for gRPC health checks
ARG GRPC_HEALTH_PROBE_VERSION=v0.4.34
RUN wget -qO /usr/local/bin/grpc_health_probe \
      "https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64" && \
    chmod +x /usr/local/bin/grpc_health_probe

ARG user=app
ARG group=docker
RUN addgroup -S $group && adduser -S -G $group $user
USER $user
WORKDIR /usr/local/service

# Copy the binary and set execute permissions
COPY --chown=$user:$group --from=build /app/chargeflow-registry /usr/local/bin/chargeflow-registry
RUN chmod +x /usr/local/bin/chargeflow-registry

# Add a health check
HEALTHCHECK --interval=5s --timeout=3s --retries=3 \
    CMD curl --fail http://localhost:80/healthz || exit 1

CMD ["/usr/local/bin/chargeflow-registry"]

FROM alpine:3.21 AS migrate

RUN apk add --no-cache ca-certificates

ARG user=app
ARG group=docker
RUN addgroup -S $group && adduser -S -G $group $user
USER $user
WORKDIR /usr/local/service

COPY --chown=$user:$group --from=build /app/migrate /usr/local/bin/migrate
RUN chmod +x /usr/local/bin/migrate

CMD ["/usr/local/bin/migrate"]