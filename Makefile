.PHONY: test clean

test: clean
	@echo "Running tests..."
	@go test -v -race -p 1 -failfast -cover -coverprofile=coverage.txt -coverpkg=./... -timeout 10m ./... && echo "Tests completed successfully"

clean:
	@if command -v kurtosis >/dev/null 2>&1; then \
		kurtosis enclave rm ethcore-test --force 2>/dev/null || true; \
	fi
