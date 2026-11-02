.DEFAULT_GOAL := build

.PHONY: build release frontend frontend-deps check fmt test test-integration test-live test-race test-preview vet

frontend/node_modules/.package-lock.json: frontend/package.json frontend/package-lock.json
	npm --prefix frontend ci

frontend-deps: frontend/node_modules/.package-lock.json

frontend: frontend-deps
	npm --prefix frontend run build

build: frontend
	go build -o build/uruflow ./cmd/uruflow-server
	go build -o build/uruflow-agent ./cmd/uruflow-agent

release: frontend
	sh packaging/release.sh

fmt:
	gofmt -w cmd internal pkg
	npm --prefix frontend run format

test: frontend-deps
	go test ./...
	npm --prefix frontend test

test-integration:
	URUFLOW_DOCKER_TESTS=1 go test -tags=integration ./...

test-live:
	URUFLOW_DOCKER_TESTS=1 go test -tags=live ./internal/docker ./internal/registry ./internal/agent/runner

test-race:
	go test -race ./...

test-preview: frontend
	URUFLOW_BROWSER_QA=1 go test -tags=integration ./internal/web -run '^TestBrowserPreview$$' -count=1 -v -timeout=35m

vet:
	go vet ./...

check: frontend-deps
	@unformatted="$$(gofmt -l cmd internal pkg)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	go vet ./...
	go test ./...
	npm --prefix frontend run check
	npm --prefix frontend test
	npm --prefix frontend run format:check
