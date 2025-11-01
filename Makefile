.PHONY: help build test lint run docker-build docker-up docker-down migrate clean

help: ## Показать эту справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# ============================================================================
# Development
# ============================================================================

build: ## Собрать server binary
	@echo "Building server..."
	cd server && go build -o ../bin/monitoring-server .

build-agent: ## Собрать agent binary (Windows)
	@echo "Building agent for Windows..."
	cd agent && GOOS=windows GOARCH=amd64 go build -o ../bin/employee-agent.exe .

test: ## Запустить тесты
	@echo "Running tests..."
	cd server && go test -v -race -coverprofile=coverage.out ./...
	@echo "Coverage:"
	cd server && go tool cover -func=coverage.out

test-coverage: ## Показать test coverage в браузере
	cd server && go test -coverprofile=coverage.out ./...
	cd server && go tool cover -html=coverage.out

lint: ## Запустить linter
	@echo "Running linter..."
	cd server && golangci-lint run

fmt: ## Форматировать код
	@echo "Formatting code..."
	cd server && go fmt ./...
	cd agent && go fmt ./...
	cd zapctx && go fmt ./...

run: ## Запустить server локально
	@echo "Starting server..."
	cd server && go run main.go handlers.go handlers_screenshot.go

# ============================================================================
# Docker
# ============================================================================

docker-build: ## Собрать Docker images
	@echo "Building Docker images..."
	docker-compose build

docker-up: ## Запустить все сервисы
	@echo "Starting services..."
	docker-compose up -d

docker-down: ## Остановить все сервисы
	@echo "Stopping services..."
	docker-compose down

docker-logs: ## Показать логи
	docker-compose logs -f

docker-restart: ## Перезапустить сервисы
	@echo "Restarting services..."
	docker-compose restart

# ============================================================================
# Database
# ============================================================================

migrate: ## Применить миграции ClickHouse
	@echo "Running migrations..."
	@if [ -z "$(CLICKHOUSE_PASSWORD)" ]; then \
		echo "Using default password from config..."; \
		clickhouse-client --host=localhost --port=9000 \
			--user=monitor_user \
			--password=change_me_in_production \
			--database=monitoring \
			< clickhouse/migrations.sql; \
	else \
		clickhouse-client --host=localhost --port=9000 \
			--user=monitor_user \
			--password=$(CLICKHOUSE_PASSWORD) \
			--database=monitoring \
			< clickhouse/migrations.sql; \
	fi
	@echo "Migrations applied successfully!"

migrate-status: ## Проверить статус БД
	@echo "Checking database status..."
	clickhouse-client --host=localhost --port=9000 \
		--user=monitor_user \
		--password=change_me_in_production \
		--query="SHOW TABLES FROM monitoring"

db-shell: ## Подключиться к ClickHouse shell
	clickhouse-client --host=localhost --port=9000 \
		--user=monitor_user \
		--password=change_me_in_production \
		--database=monitoring

# ============================================================================
# Development environment
# ============================================================================

deps: ## Установить зависимости
	@echo "Installing dependencies..."
	cd server && go mod download
	cd agent && go mod download

tidy: ## Очистить зависимости
	@echo "Tidying dependencies..."
	cd server && go mod tidy
	cd agent && go mod tidy

vendor: ## Создать vendor директорию
	@echo "Vendoring dependencies..."
	cd server && go mod vendor

# ============================================================================
# Cleanup
# ============================================================================

clean: ## Очистить build артефакты
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf server/coverage.out
	rm -f server/monitoring-server
	rm -f agent/employee-agent.exe
	@echo "Clean complete!"

clean-docker: ## Удалить Docker volumes
	@echo "Removing Docker volumes..."
	docker-compose down -v
	@echo "Docker volumes removed!"

# ============================================================================
# CI/CD
# ============================================================================

ci: lint test ## Запустить CI проверки
	@echo "CI checks passed!"

pre-commit: fmt lint test ## Pre-commit hook
	@echo "Pre-commit checks passed!"

# ============================================================================
# Documentation
# ============================================================================

docs: ## Генерировать API документацию
	@echo "Generating API documentation..."
	@if command -v swag > /dev/null; then \
		cd server && swag init; \
	else \
		echo "swag not installed. Install with: go install github.com/swaggo/swag/cmd/swag@latest"; \
	fi

# ============================================================================
# Production
# ============================================================================

build-prod: ## Production build
	@echo "Building for production..."
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-w -s" \
		-a -installsuffix cgo \
		-o ../bin/monitoring-server .

docker-build-prod: ## Собрать production Docker images
	@echo "Building production Docker images..."
	docker-compose -f docker-compose.yml -f docker-compose.prod.yml build

deploy-prod: docker-build-prod ## Deploy to production
	@echo "Deploying to production..."
	docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
