# Auth Development Makefile

DOCKER_COMPOSE := docker-compose -f docker-compose.yml
PROJECT_NAME := myapp-dev

.PHONY: up down build logs shell db-shell redis-shell test lint help 

up: setup-volumes ## Start the development environment
	$(DOCKER_COMPOSE) up -d --build

down: ## Stop the development environment
	$(DOCKER_COMPOSE) down

build: ## Rebuild the development environment
	$(DOCKER_COMPOSE) build

logs: ## Show logs of all services
	$(DOCKER_COMPOSE) logs -f

shell: ## Enter the app container shell
	$(DOCKER_COMPOSE) exec app /bin/bash

db-shell: ## Enter the database shell
	$(DOCKER_COMPOSE) exec postgres psql -U postgres -d myapp

redis-shell: ## Enter the Redis shell
	$(DOCKER_COMPOSE) exec redis redis-cli

test: ## Run tests inside the container
	$(DOCKER_COMPOSE) exec app go test ./...

lint: ## Run linter inside the container
	$(DOCKER_COMPOSE) exec app golangci-lint run

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'



setup-volumes:	## Setup Redis volume for data persistence - Run this before running up - Sets permissions for redis-data
	@mkdir -p ../../redis-data ../../pg-data
	@sudo chmod 777 ../../redis-data ../../pg-data