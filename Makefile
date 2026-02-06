VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w \
	-X github.com/heysarver/devinfra/internal/version.Version=$(VERSION) \
	-X github.com/heysarver/devinfra/internal/version.Commit=$(COMMIT) \
	-X github.com/heysarver/devinfra/internal/version.Date=$(DATE)

.PHONY: build install test lint clean

build: ## Build the devinfra binary
	go build -ldflags "$(LDFLAGS)" -o bin/devinfra .

install: build ## Install devinfra and di to GOPATH/bin
	@mkdir -p $(shell go env GOPATH)/bin
	cp bin/devinfra $(shell go env GOPATH)/bin/devinfra
	ln -sf $(shell go env GOPATH)/bin/devinfra $(shell go env GOPATH)/bin/di
	@GOBIN="$(shell go env GOPATH)/bin"; \
	if echo "$$PATH" | tr ':' '\n' | grep -qx "$$GOBIN"; then \
		echo "$$GOBIN is already in PATH"; \
	else \
		SHELL_RC=""; \
		case "$$SHELL" in \
			*/zsh)  SHELL_RC="$$HOME/.zshrc" ;; \
			*/bash) \
				if [ -f "$$HOME/.bash_profile" ]; then \
					SHELL_RC="$$HOME/.bash_profile"; \
				else \
					SHELL_RC="$$HOME/.bashrc"; \
				fi ;; \
			*)      SHELL_RC="$$HOME/.profile" ;; \
		esac; \
		echo "export PATH=\"$$GOBIN:\$$PATH\"" >> "$$SHELL_RC"; \
		echo "Added $$GOBIN to PATH in $$SHELL_RC"; \
		echo "Run 'source $$SHELL_RC' or open a new terminal to use devinfra"; \
	fi

test: ## Run tests
	go test ./...

lint: ## Run linter
	golangci-lint run

clean: ## Remove build artifacts
	rm -rf bin/

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
