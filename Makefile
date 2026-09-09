# Developer entrypoints for the payment orchestrator.
# Run `make help` for the list.

.DEFAULT_GOAL := help
.PHONY: help up down logs build api-run worker-run api-test api-tidy web-dev web-build

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

up: ## Start the full stack (postgres, redis, api, worker)
	docker compose up --build -d

down: ## Stop the stack
	docker compose down

logs: ## Tail stack logs
	docker compose logs -f

build: ## Build all docker images
	docker compose build

api-run: ## Run the api locally (needs postgres/redis running)
	cd api && go run ./cmd/api

worker-run: ## Run the worker locally
	cd api && go run ./cmd/worker

api-test: ## Run backend tests
	cd api && go test ./...

api-tidy: ## Tidy go modules
	cd api && go mod tidy

web-dev: ## Run the Next.js dev server
	cd web && pnpm dev

web-build: ## Build the frontend
	cd web && pnpm build
