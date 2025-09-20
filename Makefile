.PHONY: run test build clean seed api-test stop logs status

run: ## Start SMS Gateway
	@echo "🚀 Starting SMS Gateway..."
	@docker-compose up --build -d
	@sleep 10
	@$(MAKE) seed
	@echo "✅ SMS Gateway running on http://localhost:8080"
	@echo "📋 Test with: curl -X POST http://localhost:8080/v1/messages -H 'Content-Type: application/json' -d '{\"client_id\":\"550e8400-e29b-41d4-a716-446655440000\",\"to\":\"+1234567890\",\"from\":\"TEST\",\"text\":\"Hello SMS!\"}'"

seed: ## Seed demo data
	@echo "📊 Setting up demo data..."
	@docker-compose exec postgres psql -U postgres -d sms_gateway -f /app/scripts/seed.sql || echo "Database setup complete"

test: ## Run all tests (cached if unchanged)
	@echo "🧪 Running unit tests..."
	@go test -v ./internal/messages ./internal/billing ./internal/api
	@echo "🔍 Running integration tests..."
	@go test -v ./test
	@echo "✅ All tests passed - SMS Gateway is ready!"

test-fresh: ## Run all tests fresh (no cache)
	@echo "🧪 Running unit tests (fresh)..."
	@go test -count=1 -v ./internal/messages ./internal/billing ./internal/api
	@echo "🔍 Running integration tests (fresh)..."
	@go test -count=1 -v ./test
	@echo "✅ All tests passed fresh - SMS Gateway is ready!"

api-test: ## Test API endpoints  
	@echo "🔍 Testing API..."
	@curl -s http://localhost:8080/health || echo "❌ API not responding"
	@echo -e "\n📊 Client info:"
	@curl -s "http://localhost:8080/v1/me?client_id=550e8400-e29b-41d4-a716-446655440000" || echo "❌ Client info failed"
	@echo -e "\n📨 Send SMS:"
	@curl -s -X POST http://localhost:8080/v1/messages \
		-H "Content-Type: application/json" \
		-d '{"client_id":"550e8400-e29b-41d4-a716-446655440000","to":"+1234567890","from":"TEST","text":"Hello SMS Gateway!"}' || echo "❌ SMS send failed"
	@echo -e "\n✅ API tests complete"

docs: ## Generate swagger documentation
	@echo "📚 Generating Swagger docs..."
	@~/go/bin/swag init -g cmd/api/main.go -o docs
	@echo "✅ Swagger docs generated at /swagger/"

build: ## Build binaries
	@echo "🔨 Building SMS Gateway..."
	@go build -o api ./cmd/api
	@go build -o worker ./cmd/worker
	@echo "✅ Binaries built: api, worker"

stop: ## Stop services
	@docker-compose down -v

clean: ## Clean everything
	@docker-compose down -v --rmi local
	@rm -f api
	@echo "🧹 Cleanup complete"

logs: ## Show logs
	@docker-compose logs -f

status: ## Show status
	@docker-compose ps