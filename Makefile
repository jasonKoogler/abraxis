.DEFAULT_GOAL := help

.PHONY: build-all test-all swagger-all proto clean help

build-all: ## Build both services
	$(MAKE) -C aegis build
	$(MAKE) -C prism build

test-all: ## Run tests for all modules
	$(MAKE) -C aegis test
	$(MAKE) -C prism test

swagger-all: ## Regenerate swagger docs for both services
	$(MAKE) -C aegis swagger
	$(MAKE) -C prism swagger

proto: ## Regenerate protobuf code
	$(MAKE) -C aegis proto

clean: ## Clean build artifacts
	$(MAKE) -C aegis clean
	$(MAKE) -C prism clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
