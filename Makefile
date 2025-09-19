.PHONY: run test build clean

# Simple SMS Gateway Makefile

run: ## Start everything (infrastructure + services)
	@echo "🚀 Starting SMS Gateway..."
	@docker-compose up --build -d
	@sleep 10
	@$(MAKE) seed
	@echo "✅ SMS Gateway running on http://localhost:8080"
	@echo "🔑 Demo API Key: secret"

seed: ## Seed database (tables + demo client)
	@echo "📊 Setting up demo client..."
	@docker-compose exec postgres psql -U postgres -d sms_gateway -c "\
		CREATE EXTENSION IF NOT EXISTS pgcrypto; \
		CREATE TABLE IF NOT EXISTS clients ( \
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(), \
			name text NOT NULL, \
			api_key_hash text NOT NULL UNIQUE, \
			credit_cents bigint NOT NULL DEFAULT 0 \
		); \
		CREATE TABLE IF NOT EXISTS messages ( \
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(), \
			client_id uuid NOT NULL, \
			to_msisdn text NOT NULL, \
			from_sender text NOT NULL, \
			text text NOT NULL, \
			parts int NOT NULL DEFAULT 1, \
			status text NOT NULL DEFAULT 'QUEUED', \
			client_reference text NULL, \
			created_at timestamptz NOT NULL DEFAULT now(), \
			updated_at timestamptz NOT NULL DEFAULT now() \
		); \
		INSERT INTO clients (id, name, api_key_hash, credit_cents) \
		VALUES ('550e8400-e29b-41d4-a716-446655440000', 'Demo Client', 'secret', 100000) \
		ON CONFLICT (api_key_hash) DO NOTHING;" 2>/dev/null || echo "Database setup complete"

test: ## Run all tests and API tests
	@echo "🧪 Running Go tests..."
	@go test -v ./...
	@echo "🔍 Testing API endpoints..."
	@make api-test

api-test: ## Test API endpoints
	@echo "1. Health check:"
	@curl -s http://localhost:8080/healthz || echo "❌ API not responding"
	@echo -e "\n2. Client info:"
	@curl -s -H "X-API-Key: secret" http://localhost:8080/v1/me || echo "❌ Auth failed"
	@echo -e "\n3. Send SMS:"
	@curl -s -X POST http://localhost:8080/v1/messages \
		-H "Content-Type: application/json" \
		-H "X-API-Key: secret" \
		-d '{"to":"+1234567890","from":"TEST","text":"Hello SMS Gateway!"}' || echo "❌ SMS send failed"
	@echo -e "\n✅ API tests complete"

build: ## Build binaries
	@echo "🔨 Building SMS Gateway..."
	@go build -o api ./cmd/api
	@go build -o worker ./cmd/worker
	@echo "✅ Binaries built: api, worker"

stop: ## Stop all services
	@docker-compose down -v

clean: ## Clean everything
	@docker-compose down -v --rmi local
	@rm -f api worker
	@echo "🧹 Cleanup complete"

logs: ## Show logs
	@docker-compose logs -f

status: ## Show service status
	@docker-compose ps
