# SMS Gateway - System Design

## 🎯 Overview

A modern, high-performance SMS Gateway designed for reliability, scalability, and ease of use. The system handles SMS message processing, OTP generation, express delivery, and comprehensive reporting without requiring authentication.

## 🏛️ System Architecture

### High-Level Architecture
```
┌─────────────────┐
│   Load Balancer │
└─────────────────┘
         │
┌─────────────────┐    ┌─────────────────┐
│   API Gateway   │────│   SMS Gateway   │
│   (Optional)    │    │   API Service   │
└─────────────────┘    └─────────────────┘
                                │
    ┌───────────────────────────┼───────────────────────────┐
    │                           │                           │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   PostgreSQL    │    │      Redis      │    │      NATS       │
│   (Messages)    │    │    (Cache)      │    │   (Queue)       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                                               ┌─────────────────┐
                                               │  SMS Providers  │
                                               │     (Mock)      │
                                               └─────────────────┘
```

### Component Responsibilities

#### API Service (`cmd/api`)
- **Purpose**: HTTP API server handling all client requests
- **Framework**: Fiber (high-performance HTTP framework)
- **Features**:
  - REST API endpoints
  - Request validation
  - Response formatting
  - Health checks
  - Swagger documentation

#### Message Management (`internal/messages`)
- **Purpose**: Core message processing logic
- **Components**:
  - `models.go`: Data structures and validation
  - `store.go`: Database operations
- **Features**:
  - SMS part calculation (GSM7/UCS2)
  - Message lifecycle management
  - Status tracking

#### Billing System (`internal/billing`)
- **Purpose**: Credit management and financial operations
- **Features**:
  - Credit hold/capture/release pattern
  - Transaction safety
  - Cost calculation
  - Express delivery surcharges

#### Delivery Processing (`internal/delivery`)
- **Purpose**: Handle delivery receipts (DLR) from providers
- **Features**:
  - DLR webhook processing
  - Status updates
  - Credit finalization

#### Database Layer (`internal/db`)
- **Purpose**: Database connection management
- **Components**:
  - PostgreSQL connection
  - Redis connection
  - Migration support

#### Message Queue (`internal/messaging`)
- **Purpose**: Asynchronous message processing
- **Technology**: NATS
- **Features**:
  - Message queuing
  - Retry handling
  - Dead letter queue

#### Provider Integration (`internal/providers`)
- **Purpose**: SMS provider abstraction
- **Current**: Mock provider for testing
- **Extensible**: Easy to add real providers

## 🗄️ Data Models

### Message
```go
type Message struct {
    ID                uuid.UUID `json:"id"`
    ClientID          uuid.UUID `json:"client_id"`
    To                string    `json:"to"`
    From              string    `json:"from"`
    Text              string    `json:"text"`
    Parts             int       `json:"parts"`
    Status            Status    `json:"status"`
    Reference         *string   `json:"reference,omitempty"`
    Provider          *string   `json:"provider,omitempty"`
    ProviderMessageID *string   `json:"provider_message_id,omitempty"`
    Attempts          int       `json:"attempts"`
    LastError         *string   `json:"last_error,omitempty"`
    Express           bool      `json:"express"`
    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}
```

### Message Status Flow
```
QUEUED → SENDING → SENT → DELIVERED
   ↓        ↓        ↓        
FAILED_TEMP → FAILED_PERM
```

### Credit Lock
```go
type CreditLock struct {
    ID        uuid.UUID `json:"id"`
    ClientID  uuid.UUID `json:"client_id"`
    MessageID uuid.UUID `json:"message_id"`
    Amount    int64     `json:"amount"`
    State     string    `json:"state"` // HELD, CAPTURED, RELEASED
}
```

## 🔄 Message Processing Flow

### 1. Message Submission
```
Client Request → Validation → Cost Calculation → Message Creation → Credit Hold → Queue
```

### 2. Message Processing
```
Queue → Provider Send → Status Update → DLR Webhook → Final Status → Credit Finalization
```

### 3. Credit Flow
```
Available Credits → Hold (Deduct) → Capture (Finalize) OR Release (Return)
```

## 🔧 API Design

### RESTful Principles
- **Resource-based URLs**: `/v1/messages`, `/v1/me`
- **HTTP Methods**: GET, POST for appropriate operations
- **Status Codes**: Meaningful HTTP status codes
- **JSON**: Consistent JSON request/response format

### Request/Response Examples

#### Send Message Request
```json
{
  "client_id": "550e8400-e29b-41d4-a716-446655440000",
  "to": "+1234567890",
  "from": "SENDER",
  "text": "Hello World",
  "reference": "order-123",
  "otp": false,
  "express": false
}
```

#### Send Message Response
```json
{
  "message_id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "QUEUED",
  "otp_code": "123456"
}
```

### Error Handling
```json
{
  "error": "insufficient credits",
  "required": 10
}
```

## 💾 Database Design

### Tables

#### messages
```sql
CREATE TABLE messages (
    id UUID PRIMARY KEY,
    client_id UUID NOT NULL,
    to_msisdn VARCHAR(20) NOT NULL,
    from_sender VARCHAR(20) NOT NULL,
    text TEXT NOT NULL,
    parts INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL,
    client_reference VARCHAR(100),
    provider VARCHAR(50),
    provider_message_id VARCHAR(100),
    attempts INTEGER DEFAULT 0,
    last_error TEXT,
    express BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
```

#### clients
```sql
CREATE TABLE clients (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    api_key_hash VARCHAR(255) NOT NULL,
    credit_cents BIGINT NOT NULL DEFAULT 0,
    dlr_callback_url VARCHAR(500),
    callback_hmac_secret VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### credit_locks
```sql
CREATE TABLE credit_locks (
    id UUID PRIMARY KEY,
    client_id UUID NOT NULL REFERENCES clients(id),
    message_id UUID NOT NULL REFERENCES messages(id),
    amount_cents BIGINT NOT NULL,
    state VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### Indexes
```sql
CREATE INDEX idx_messages_client_id ON messages(client_id);
CREATE INDEX idx_messages_status ON messages(status);
CREATE INDEX idx_messages_provider_msg_id ON messages(provider_message_id);
CREATE INDEX idx_credit_locks_message_id ON credit_locks(message_id);
```

## 🚀 Scalability Considerations

### Horizontal Scaling
- **Stateless API**: No session state in API servers
- **Database Connection Pooling**: Efficient database connections
- **Message Queue**: NATS for distributed processing
- **Load Balancing**: Multiple API instances

### Performance Optimizations
- **Connection Pooling**: Database and Redis connections
- **Async Processing**: Message queue for heavy operations
- **Caching**: Redis for frequently accessed data
- **Indexing**: Optimized database queries

### Capacity Planning
- **100 TPS**: Target throughput
- **100M messages/day**: Daily volume capacity
- **Database Partitioning**: By date for large volumes
- **Archive Strategy**: Move old messages to cold storage

## 🔒 Security

### Data Protection
- **Input Validation**: All API inputs validated
- **SQL Injection Prevention**: Parameterized queries
- **Rate Limiting**: Per-client request limits (future)
- **TLS**: Encrypted connections in production

### Authentication (Removed)
- **No Authentication**: As per requirements
- **Client ID**: Simple client identification
- **Future**: Can add API key authentication if needed

## 📊 Monitoring & Observability

### Logging
- **Structured Logging**: JSON format with slog
- **Request Tracing**: Request ID tracking
- **Error Logging**: Comprehensive error information
- **Performance Logging**: Response times

### Health Checks
- **Liveness**: `/health` endpoint
- **Readiness**: `/ready` with dependency checks
- **Database Health**: Connection testing

### Metrics (Future)
- **Request Metrics**: Rate, latency, errors
- **Business Metrics**: Messages sent, delivery rates
- **System Metrics**: CPU, memory, connections

## 🧪 Testing Strategy

### Test Pyramid
```
    ┌─────────────┐
    │     E2E     │  ← Full API workflow tests
    ├─────────────┤
    │ Integration │  ← Database, queue, external services
    ├─────────────┤
    │    Unit     │  ← Business logic, utilities
    └─────────────┘
```

### E2E Test Coverage
- **API Endpoints**: All REST endpoints
- **Business Flows**: Complete message lifecycle
- **Error Scenarios**: Invalid requests, failures
- **Edge Cases**: Boundary conditions

### Test Environment
- **Isolated**: Separate test database
- **Repeatable**: Tests can run multiple times
- **Fast**: Quick feedback loop
- **Comprehensive**: High coverage

## 🚢 Deployment

### Container Strategy
- **Docker**: Containerized application
- **Docker Compose**: Local development
- **Multi-stage Build**: Optimized images

### Environment Management
- **Configuration**: Environment variables
- **Secrets**: Secure credential management
- **Database Migrations**: Automated schema updates

### Production Deployment
- **Health Checks**: Kubernetes readiness/liveness probes
- **Rolling Updates**: Zero-downtime deployments
- **Resource Limits**: CPU and memory constraints
- **Monitoring**: Application and infrastructure monitoring

## 🔮 Future Enhancements

### Short Term
- **Real SMS Providers**: Twilio, AWS SNS integration
- **Webhook Callbacks**: Client notification system
- **Rate Limiting**: Per-client request throttling
- **Metrics Dashboard**: Operational visibility

### Medium Term
- **Multi-tenancy**: Isolated client environments
- **Message Templates**: Predefined message formats
- **Scheduling**: Delayed message sending
- **A/B Testing**: Provider performance comparison

### Long Term
- **Global Distribution**: Multi-region deployment
- **Machine Learning**: Delivery optimization
- **Advanced Analytics**: Business intelligence
- **Self-healing**: Automated failure recovery

---

This design provides a solid foundation for a production-grade SMS Gateway system that can scale to handle high volumes while maintaining reliability and performance.