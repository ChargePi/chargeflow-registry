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

RUN --mount=type=secret,id=gh_token,required=true \
    --mount=type=cache,target="/root/.cache/go-build" \
    git config --global url."https://x-access-token:$(cat /run/secrets/gh_token)@github.com/".insteadOf "https://github.com/" && \
    go mod download && go mod verify

# Copy the rest of the source code
COPY . .

RUN --mount=type=cache,target="/root/.cache/go-build" go build -o ../chargeflow-registry ./cmd/app

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