.PHONY: build check fmt test vet

build:
	go build ./...

fmt:
	gofmt -w .

test:
	go test ./...

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
