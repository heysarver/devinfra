PROJECT_NAME := {{.ProjectName}}

# Auto-discover all compose files: base + any flavor overlays
# $(sort) ensures deterministic, alphabetical ordering
# $(wildcard) is Make-native (no shell dependency)
COMPOSE_FILES := $(sort $(wildcard docker-compose*.yaml))
COMPOSE_FLAGS := $(patsubst %,-f %,$(COMPOSE_FILES))

.PHONY: help up down restart logs ps

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

up: ## Start this project
	docker compose $(COMPOSE_FLAGS) up -d

down: ## Stop this project
	docker compose $(COMPOSE_FLAGS) down

restart: ## Restart this project
	docker compose $(COMPOSE_FLAGS) restart

logs: ## Tail logs
	docker compose $(COMPOSE_FLAGS) logs -f

ps: ## Show running containers
	docker compose $(COMPOSE_FLAGS) ps
