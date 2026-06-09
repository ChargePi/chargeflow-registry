SERVICE_NAME := chargeflow-registry
GO           := go
GOFLAGS      :=

.PHONY: build run test lint generate proto mocks clean

build:
	$(GO) build $(GOFLAGS) -o bin/$(SERVICE_NAME) ./cmd/app

run:
	$(GO) run ./cmd/app

test:
	$(GO) test -v -short ./... -coverpkg=./... -coverprofile=unit_coverage.out

test-integration:
	$(GO) test -v -run 'Integration$$' ./... -coverpkg=./... -coverprofile=integration_coverage.out

lint:
	golangci-lint run --timeout 2m

generate: proto mocks

proto:
	buf generate

mocks:
	mockery

clean:
	rm -rf bin/ unit_coverage.out integration_coverage.out