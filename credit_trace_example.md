
// Test credit tracing
func TestCreditTracing() {
    clientID := "550e8400-e29b-41d4-a716-446655440000"
    
    // 1. Client sends SMS request with their ID
    curl -X POST http://localhost:8080/v1/messages \
      -H "Content-Type: application/json" \
      -d "{\"client_id\":\"$clientID\",\"to\":\"+1234567890\",\"from\":\"TEST\",\"text\":\"Hello\"}"
      
    // 2. System decreases credits for THIS specific client
    // 3. Creates audit trail linking client -> message -> credit deduction
}
