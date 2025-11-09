.PHONY: format format-check lint lint-fix build test

# Format all code (Go, JS, HTML, CSS, JSON)
format:
	@echo "Formatting Go code..."
	gofmt -w .
	@echo "Formatting JavaScript, HTML, CSS, JSON..."
	cd web && npm run format

# Check formatting without modifying files
format-check:
	@echo "Checking Go code formatting..."
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following Go files need formatting:"; \
		gofmt -l .; \
		exit 1; \
	fi
	@echo "Checking JavaScript, HTML, CSS, JSON formatting..."
	cd web && npm run format:check

# Lint JavaScript code
lint:
	@echo "Linting JavaScript code..."
	cd web && npm run lint

# Lint and fix JavaScript code
lint-fix:
	@echo "Linting and fixing JavaScript code..."
	cd web && npm run lint:fix

# Build the project
build:
	./build.sh

# Run tests
test:
	go test ./...
