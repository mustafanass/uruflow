.PHONY: build check fmt test test-integration test-live test-preview test-race vet

build:
	go build ./...

fmt:
	gofmt -w .

test:
	go test ./...

test-integration:
	URUFLOW_DOCKER_TESTS=1 go test -tags=integration ./...

test-live:
	URUFLOW_DOCKER_TESTS=1 go test -tags=live ./internal/docker ./internal/registry ./internal/agent/runner

test-preview:
	go test -tags=preview -run TestPreview -v ./internal/cliui ./internal/workbench

test-race:
	go test -race ./...

vet:
	go vet ./...

check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	go vet ./...
	go test ./...
