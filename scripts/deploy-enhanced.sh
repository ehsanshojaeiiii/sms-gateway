#!/bin/bash

# SMS Gateway Enhanced Deployment Script
# Safely switches between simple and enhanced worker modes

set -e

# Configuration
WORKER_MODE="${WORKER_MODE:-enhanced}"
POOL_SIZE="${POOL_SIZE:-0}"  # 0 = auto-detect
BATCH_SIZE="${BATCH_SIZE:-50}"
BUFFER_SIZE="${BUFFER_SIZE:-1000}"
ENABLE_METRICS="${ENABLE_METRICS:-true}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 SMS Gateway Enhanced Deployment${NC}"
echo -e "${BLUE}===================================${NC}"
echo ""
echo "Configuration:"
echo "  • Worker Mode: $WORKER_MODE"
echo "  • Pool Size: $POOL_SIZE"
echo "  • Batch Size: $BATCH_SIZE"
echo "  • Buffer Size: $BUFFER_SIZE"
echo "  • Enable Metrics: $ENABLE_METRICS"
echo ""

# Function to check if system is ready
check_system_ready() {
    echo -e "${YELLOW}🔍 Checking system readiness...${NC}"
    
    # Check if current system is running
    if ! curl -s http://localhost:8080/health > /dev/null; then
        echo -e "${RED}❌ SMS Gateway not running. Please start with: make run${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✅ System is ready${NC}"
}

# Function to backup current configuration
backup_current_config() {
    echo -e "${YELLOW}📦 Backing up current configuration...${NC}"
    
    # Create backup directory
    mkdir -p ./backups/$(date +%Y%m%d_%H%M%S)
    
    # Backup docker-compose.yml
    cp docker-compose.yml ./backups/$(date +%Y%m%d_%H%M%S)/
    
    echo -e "${GREEN}✅ Configuration backed up${NC}"
}

# Function to update docker-compose for enhanced mode
update_docker_compose() {
    echo -e "${YELLOW}🔧 Updating Docker Compose configuration...${NC}"
    
    # Create enhanced docker-compose.yml
    cat > docker-compose.enhanced.yml << EOF
# SMS Gateway Docker Compose - Enhanced Mode
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: sms_gateway
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts:/app/scripts
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5

  nats:
    image: nats:2-alpine
    ports:
      - "4222:4222"
    healthcheck:
      test: ["CMD-SHELL", "nc -z localhost 4222"]
      interval: 10s
      timeout: 3s
      retries: 5

  api:
    build:
      context: .
      dockerfile: docker/Dockerfile.api
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - POSTGRES_URL=postgres://postgres:password@postgres:5432/sms_gateway?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - NATS_URL=nats://nats:4222
      - PRICE_PER_PART_CENTS=5
      - EXPRESS_SURCHARGE_CENTS=2
      - LOG_LEVEL=info
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      nats:
        condition: service_healthy

  worker:
    build:
      context: .
      dockerfile: docker/Dockerfile.worker
    environment:
      - POSTGRES_URL=postgres://postgres:password@postgres:5432/sms_gateway?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - NATS_URL=nats://nats:4222
      - PRICE_PER_PART_CENTS=5
      - EXPRESS_SURCHARGE_CENTS=2
      - LOG_LEVEL=info
      # Enhanced worker configuration
      - WORKER_MODE=$WORKER_MODE
      - WORKER_POOL_SIZE=$POOL_SIZE
      - WORKER_BATCH_SIZE=$BATCH_SIZE
      - WORKER_BUFFER_SIZE=$BUFFER_SIZE
      - WORKER_ENABLE_METRICS=$ENABLE_METRICS
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      nats:
        condition: service_healthy

volumes:
  postgres_data:
EOF

    echo -e "${GREEN}✅ Docker Compose configuration updated${NC}"
}

# Function to build enhanced worker
build_enhanced_worker() {
    echo -e "${YELLOW}🔨 Building enhanced worker...${NC}"
    
    # Build both versions
    go build -o worker-simple ./cmd/worker
    go build -o worker-enhanced ./cmd/enhanced_worker
    
    # Update Dockerfile to use enhanced worker
    cat > docker/Dockerfile.worker.enhanced << EOF
# Build stage
FROM golang:1.25.1-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build enhanced worker
RUN go build -o worker ./cmd/enhanced_worker

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Copy the enhanced worker binary
COPY --from=builder /app/worker .

# Command to run
CMD ["./worker"]
EOF

    echo -e "${GREEN}✅ Enhanced worker built${NC}"
}

# Function to deploy enhanced system
deploy_enhanced_system() {
    echo -e "${YELLOW}🚀 Deploying enhanced system...${NC}"
    
    # Stop current system
    docker-compose down
    
    # Deploy with enhanced configuration
    docker-compose -f docker-compose.enhanced.yml up --build -d
    
    # Wait for system to be ready
    echo -e "${YELLOW}⏳ Waiting for system to be ready...${NC}"
    sleep 15
    
    # Seed database
    docker-compose -f docker-compose.enhanced.yml exec postgres psql -U postgres -d sms_gateway -f /app/scripts/seed.sql || echo "Database already seeded"
    
    echo -e "${GREEN}✅ Enhanced system deployed${NC}"
}

# Function to test enhanced system
test_enhanced_system() {
    echo -e "${YELLOW}🧪 Testing enhanced system...${NC}"
    
    # Health check
    if curl -s http://localhost:8080/health > /dev/null; then
        echo -e "${GREEN}✅ API health check passed${NC}"
    else
        echo -e "${RED}❌ API health check failed${NC}"
        return 1
    fi
    
    # Quick performance test
    echo -e "${YELLOW}📊 Running quick performance test...${NC}"
    time bash -c 'for i in {1..100}; do curl -s -X POST http://localhost:8080/v1/messages -H "Content-Type: application/json" -d "{\"client_id\":\"550e8400-e29b-41d4-a716-446655440000\",\"to\":\"+1enhanced$i\",\"from\":\"ENHANCED\",\"text\":\"Enhanced test #$i\"}" > /dev/null & done; wait'
    
    echo -e "${GREEN}✅ Enhanced system test completed${NC}"
}

# Function to rollback if needed
rollback_system() {
    echo -e "${YELLOW}🔄 Rolling back to simple worker...${NC}"
    
    docker-compose -f docker-compose.enhanced.yml down
    docker-compose up -d
    
    echo -e "${GREEN}✅ Rollback completed${NC}"
}

# Main execution
main() {
    case "${1:-deploy}" in
        "check")
            check_system_ready
            ;;
        "backup")
            backup_current_config
            ;;
        "build")
            build_enhanced_worker
            ;;
        "deploy")
            check_system_ready
            backup_current_config
            update_docker_compose
            build_enhanced_worker
            deploy_enhanced_system
            test_enhanced_system
            ;;
        "test")
            test_enhanced_system
            ;;
        "rollback")
            rollback_system
            ;;
        "help"|*)
            echo "Usage: $0 [command]"
            echo ""
            echo "Commands:"
            echo "  check     - Check if system is ready"
            echo "  backup    - Backup current configuration"
            echo "  build     - Build enhanced worker"
            echo "  deploy    - Full enhanced deployment (default)"
            echo "  test      - Test enhanced system"
            echo "  rollback  - Rollback to simple worker"
            echo "  help      - Show this help"
            echo ""
            echo "Environment Variables:"
            echo "  WORKER_MODE=enhanced|simple"
            echo "  POOL_SIZE=0 (auto-detect) or specific number"
            echo "  BATCH_SIZE=50"
            echo "  BUFFER_SIZE=1000"
            echo "  ENABLE_METRICS=true|false"
            ;;
    esac
}

# Run main function
main "$@"
