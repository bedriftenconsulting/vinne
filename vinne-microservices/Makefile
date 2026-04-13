.PHONY: help dev stop clean test migrate logs proto proto-check proto-clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev: ## Start all services in development mode
	docker-compose up -d
	@echo "Services starting..."
	@echo "Apollo Router (GraphQL Gateway): http://localhost:4000"
	@echo "Jaeger UI (Tracing): http://localhost:16686"
	@echo "Prometheus (Metrics): http://localhost:9090"
	@echo "Grafana (Dashboards): http://localhost:3001 (admin/admin)"
	@echo ""
	@echo "Services:"
	@echo "Admin Management Service (with Auth): gRPC port 50057, metrics http://localhost:8085"
	@echo "Agent Auth Service: http://localhost:8082"
	@echo "Player Auth Service: http://localhost:8083"

stop: ## Stop all services
	docker-compose down

clean: ## Clean up all data volumes
	docker-compose down -v

build: ## Build all services
	docker-compose build

build-service: ## Build specific service (usage: make build-service SERVICE=service-admin-auth)
	docker build -t randco/$(SERVICE):latest ./services/$(SERVICE)

deploy-service: ## Deploy specific service to K8s (usage: make deploy-service SERVICE=service-admin-auth ENV=dev|staging|production)
	@if [ "$(ENV)" = "production" ]; then \
		echo "⚠️  Production deployment requires confirmation."; \
		read -p "Deploy $(SERVICE) to PRODUCTION? (yes/no): " confirm; \
		if [ "$$confirm" != "yes" ]; then echo "Deployment cancelled."; exit 1; fi; \
	fi
	@echo "Building $(SERVICE) for $(ENV)..."
	docker build -t randco/$(SERVICE):$(ENV)-$(shell git rev-parse --short HEAD) ./services/$(SERVICE)
	@echo "Pushing to registry..."
	docker push randco/$(SERVICE):$(ENV)-$(shell git rev-parse --short HEAD)
	@echo "Deploying to $(ENV)..."
	kubectl set image deployment/$(SERVICE) $(SERVICE)=randco/$(SERVICE):$(ENV)-$(shell git rev-parse --short HEAD) -n randco-$(ENV)
	kubectl rollout status deployment/$(SERVICE) -n randco-$(ENV)

deploy-all: ## Deploy all services that have changes (usage: make deploy-all ENV=dev|staging)
	@if [ "$(ENV)" = "production" ]; then \
		echo "❌ Bulk production deployment not allowed. Deploy services individually."; \
		exit 1; \
	fi
	@for service in $$(ls services/); do \
		if git diff --name-only HEAD~1 HEAD | grep -q "services/$$service"; then \
			echo "Deploying $$service to $(ENV)..."; \
			make deploy-service SERVICE=$$service ENV=$(ENV); \
		fi; \
	done

# Environment-specific commands
dev-local: ## Start local development environment
	@echo "Starting local development environment..."
	docker-compose up -d
	@echo "Local environment ready at http://localhost:4000"

deploy-dev: ## Deploy to dev environment (auto-triggered on develop branch)
	@echo "Deploying to DEV environment..."
	make deploy-all ENV=dev

deploy-staging: ## Deploy to staging environment (auto-triggered on main branch)
	@echo "Deploying to STAGING environment..."
	make deploy-all ENV=staging

deploy-production: ## Deploy to production (manual, requires tag)
	@if [ -z "$$(git describe --exact-match --tags 2>/dev/null)" ]; then \
		echo "❌ Production deployment requires a git tag."; \
		exit 1; \
	fi
	@echo "Production deployment must be done service by service."
	@echo "Use: make deploy-service SERVICE=<service-name> ENV=production"

# Environment status commands
status-dev: ## Check dev environment status
	kubectl get pods -n randco-dev
	kubectl get svc -n randco-dev

status-staging: ## Check staging environment status
	kubectl get pods -n randco-staging
	kubectl get svc -n randco-staging

status-production: ## Check production environment status
	kubectl get pods -n randco-production
	kubectl get svc -n randco-production

restart: ## Restart all services
	docker-compose restart

logs: ## Tail logs for all services
	docker-compose logs -f

logs-service: ## Tail logs for a specific service (usage: make logs-service SERVICE=service-admin-auth)
	docker-compose logs -f $(SERVICE)

ps: ## Show running services
	docker-compose ps

migrate-all: ## Run migrations for all services
	@echo "Running migrations for all services..."
	@for service in service-admin-management service-agent-auth service-agent-management; do \
		echo "Migrating $$service..."; \
		cd services/$$service && goose -dir migrations postgres "postgresql://localhost:543X/$$service" up; \
	done

proto: ## Generate all protobuf code
	@echo "🔧 Generating protobuf code..."
	@./scripts/generate-protos.sh
	@echo "✅ Proto generation complete!"

proto-check: ## Check if protobuf code is up to date
	@echo "🔍 Checking if protobuf code is up to date..."
	@mkdir -p /tmp/proto-check
	@cp -r proto /tmp/proto-check/
	@./scripts/generate-protos.sh > /dev/null 2>&1
	@if ! diff -r proto /tmp/proto-check/proto > /dev/null 2>&1; then \
		echo "❌ Proto files are out of date. Run 'make proto' to regenerate."; \
		rm -rf /tmp/proto-check; \
		exit 1; \
	else \
		echo "✅ Proto files are up to date!"; \
		rm -rf /tmp/proto-check; \
	fi

proto-clean: ## Clean all generated protobuf files
	@echo "🧹 Cleaning generated protobuf files..."
	@find proto -name "*.pb.go" -type f -delete
	@echo "✅ Cleaned all generated proto files"

generate-graphql: ## Generate GraphQL code for all services
	@for service in service-agent-auth; do \
		echo "Generating GraphQL for $$service..."; \
		cd services/$$service && go run github.com/99designs/gqlgen generate; \
	done

federate: ## Compose federation supergraph
	@echo "Composing federation supergraph..."
	rover supergraph compose --config ./infrastructure/apollo/supergraph-config.yaml > ./infrastructure/apollo/supergraph.graphql

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v ./tests/integration/...

init: ## Initialize project (first time setup)
	@echo "Initializing RANDCO Microservices..."
	@mkdir -p infrastructure/apollo
	@mkdir -p infrastructure/prometheus
	@mkdir -p tests/integration
	@echo "Creating service directories with service- prefix..."
	@for service in service-admin-management service-agent-auth service-agent-management service-game service-draw service-payment service-terminal service-wallet; do \
		mkdir -p services/$$service/{cmd/server,internal/{config,models,repositories,services,handlers,events,queue,cache,middleware,common},graph,migrations,tests}; \
	done
	@echo "Project structure created!"

git-init: ## Initialize git repository with proper .gitignore
	@echo "Initializing Git repository..."
	git init
	@echo "Creating .gitignore..."
	@cat > .gitignore << 'EOF' \
	# Binaries \
	*.exe \
	*.exe~ \
	*.dll \
	*.so \
	*.dylib \
	bin/ \
	dist/ \
	\
	# Test binary, built with go test -c \
	*.test \
	\
	# Output of the go coverage tool \
	*.out \
	\
	# Go workspace file \
	go.work \
	\
	# Dependency directories \
	vendor/ \
	\
	# IDE \
	.idea/ \
	.vscode/ \
	*.swp \
	*.swo \
	*~ \
	.DS_Store \
	\
	# Environment variables \
	.env \
	.env.local \
	.env.*.local \
	\
	# Docker \
	docker-compose.override.yml \
	\
	# Logs \
	*.log \
	logs/ \
	\
	# Temporary files \
	tmp/ \
	temp/ \
	\
	# Generated files \
	*.generated.go \
	graph/generated/ \
	graph/model/models_gen.go \
	\
	# Database \
	*.db \
	*.sqlite \
	*.sqlite3 \
	\
	# Build artifacts \
	build/ \
	\
	# Air (hot reload) \
	tmp/ \
	.air.toml.tmp \
	EOF
	@echo "Git repository initialized!"

dev-admin: ## Start admin services and dependencies (auth + management)
	docker-compose up -d service-admin-auth-db service-admin-auth-redis service-admin-auth service-admin-management-db service-admin-management-redis service-admin-management kafka jaeger redis-shared

dev-agent: ## Start only agent auth service and dependencies
	docker-compose up -d service-agent-auth-db service-agent-auth-redis service-agent-auth kafka jaeger redis-shared

dev-player: ## Start only player auth service and dependencies
	docker-compose up -d kafka jaeger redis-shared