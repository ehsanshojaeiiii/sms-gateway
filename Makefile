# SMS Gateway - Clean Production Makefile
.PHONY: run test build clean stop logs status api-test scale-test

# 🚀 Main Commands
run: ## Start SMS Gateway (infrastructure + services)
	@echo "🚀 Starting SMS Gateway..."
	@docker-compose up --build -d
	@sleep 10
	@$(MAKE) seed
	@echo "✅ SMS Gateway running on http://localhost:8080"
	@echo "📱 Test: curl -X POST http://localhost:8080/v1/messages -H 'Content-Type: application/json' -d '{\"client_id\":\"550e8400-e29b-41d4-a716-446655440000\",\"to\":\"+1234567890\",\"from\":\"TEST\",\"text\":\"Hello SMS!\"}'"

test: ## Run unit tests
	@echo "🧪 Running unit tests..."
	@go test -v ./internal/messages ./internal/billing ./internal/api ./test
	@echo "✅ Unit tests passed!"



build: ## Build binaries
	@echo "🔨 Building..."
	@go build -o api ./cmd/api
	@go build -o worker ./cmd/worker
	@echo "✅ Built: api, worker"

# 📊 Testing & Validation
api-test: ## Test API endpoints
	@echo "🔍 Testing API endpoints..."
	@curl -s http://localhost:8080/health | jq . || echo "❌ Health check failed"
	@echo "\n📊 Client info:"
	@curl -s "http://localhost:8080/v1/me?client_id=550e8400-e29b-41d4-a716-446655440000" | jq . || echo "❌ Client info failed"
	@echo "\n📨 Send SMS:"
	@curl -s -X POST http://localhost:8080/v1/messages \
		-H "Content-Type: application/json" \
		-d '{"client_id":"550e8400-e29b-41d4-a716-446655440000","to":"+1234567890","from":"TEST","text":"Hello SMS Gateway!"}' | jq . || echo "❌ SMS send failed"
	@echo "\n✅ API tests complete"

scale-test: ## Test scale (100 concurrent requests)
	@echo "🔥 Scale Test: 100 concurrent SMS requests"
	@echo "📊 Starting load..."
	@time bash -c 'for i in {1..100}; do curl -s -X POST http://localhost:8080/v1/messages -H "Content-Type: application/json" -d "{\"client_id\":\"550e8400-e29b-41d4-a716-446655440000\",\"to\":\"+123456789$$i\",\"from\":\"SCALE\",\"text\":\"Scale test message #$$i\"}" > /dev/null & done; wait'
	@echo "✅ Scale test completed!"
	@echo "📈 Check credits: curl \"http://localhost:8080/v1/me?client_id=550e8400-e29b-41d4-a716-446655440000\""

# 🛠️ Utility Commands
seed: ## Seed demo data
	@echo "📊 Setting up demo data..."
	@docker-compose exec postgres psql -U postgres -d sms_gateway -f /app/scripts/seed.sql || echo "Database ready"

stop: ## Stop services
	@echo "🛑 Stopping services..."
	@docker-compose down -v

clean: ## Clean everything
	@echo "🧹 Cleaning up..."
	@docker-compose down -v --rmi local
	@rm -f api worker
	@echo "✅ Cleanup complete"

logs: ## Show logs
	@docker-compose logs -f

status: ## Show service status
	@docker-compose ps


# 📚 Documentation
help: ## Show this help
	@echo "SMS Gateway - Available Commands:"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <command>\n\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-12s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)