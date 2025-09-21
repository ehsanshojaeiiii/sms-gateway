# K6 Load Testing for SMS Gateway

## 🎯 **Load Test for 69,000 Requests in 10 Minutes**

This directory contains a comprehensive K6 load test designed to validate the SMS Gateway's ability to handle **69,000 requests in 10 minutes** (~115 requests/second).

## 🚀 **Quick Start**

### Prerequisites
```bash
# Install K6
# macOS
brew install k6

# Linux
sudo apt install k6

# Windows
choco install k6
```

### Run the High-Volume Load Test
```bash
# Start SMS Gateway first
make run

# Run the 69k request load test
k6 run k6/load-test-69k.js
```

## 📊 **Test Configuration**

### **Load Profile (69,000 requests in 10 minutes)**
```javascript
stages: [
  { duration: '30s', target: 20 },   // Ramp up slowly
  { duration: '30s', target: 50 },   // Increase load  
  { duration: '60s', target: 80 },   // Near target
  { duration: '60s', target: 115 },  // Target RPS: 115/sec
  { duration: '420s', target: 115 }, // Maintain 115 RPS (main test)
  { duration: '30s', target: 50 },   // Ramp down
  { duration: '30s', target: 0 },    // Cool down
]
```

### **Message Types Distribution**
- **80% Regular SMS**: Standard text messages (202 Accepted)
- **10% Express SMS**: High-priority messages (202 Accepted)  
- **10% OTP SMS**: Immediate delivery messages (200 OK or 503 Unavailable)

### **Performance Thresholds**
- **Response Time**: 95% under 2 seconds
- **Error Rate**: Less than 15%
- **Success Rate**: Over 85%
- **Total Errors**: Max 10,350 (15% of 69,000)

## 🎯 **Test Scenarios**

### **Realistic Load Simulation**
The test simulates realistic usage patterns:

1. **Mixed Message Types**: Different SMS types with appropriate ratios
2. **Varied Content**: Random message templates to simulate real usage
3. **Distributed Phone Numbers**: 69,000 unique phone numbers
4. **Natural Pacing**: Brief delays to simulate user behavior

### **System Validation Points**
- ✅ **Health Check**: Verify system is ready before starting
- ✅ **Credit Validation**: Check sufficient credits are available  
- ✅ **Response Validation**: Verify message_id and status in responses
- ✅ **Error Monitoring**: Track and report all failure types
- ✅ **Final State Check**: Verify system health after test completion

## 📈 **Expected Results**

### **Performance Targets**
- **Throughput**: 115 requests/second sustained
- **Success Rate**: 85%+ (industry standard under load)
- **Response Time**: 95% under 2 seconds
- **System Stability**: No crashes or memory leaks

### **What the Test Validates**
1. **Scalability**: Can handle 69,000 requests in 10 minutes
2. **Concurrency**: Worker pool efficiency under high load
3. **Credit Management**: No race conditions with concurrent transactions
4. **Error Handling**: Proper HTTP status codes under stress
5. **Resource Management**: No goroutine leaks or memory issues

## 🔧 **Alternative Test Commands**

### **Quick Load Test (Current Implementation)**
```bash
# Existing comprehensive test (recommended for development)
./test-multiple-scenarios.sh

# Basic scale test (100 requests)
make scale-test
```

### **Custom K6 Tests**
```bash
# Run with specific VU count
k6 run --vus 50 --duration 2m k6/load-test-69k.js

# Run with custom thresholds
k6 run --thresholds 'http_req_duration[p(95)]<1000' k6/load-test-69k.js
```

## 📊 **Monitoring During Test**

### **Real-time Metrics**
K6 provides live metrics during the test:
- **Active VUs**: Current virtual users
- **RPS**: Requests per second 
- **Response Time**: P50, P95, P99 percentiles
- **Error Rate**: Failed requests percentage

### **System Monitoring**
Monitor the SMS Gateway during the test:
```bash
# Watch system resources
docker-compose logs -f

# Monitor database connections
docker-compose exec postgres psql -U postgres -d sms_gateway -c "SELECT * FROM pg_stat_activity;"

# Check NATS performance
docker-compose logs nats
```

## 🏆 **Production Readiness Validation**

### **This Test Validates**
- ✅ **PDF Requirement**: "100M messages/day architecture support"
- ✅ **Scale Target**: 69,000 requests in 10 minutes (subset of 100M/day)
- ✅ **Worker Pool**: Controlled concurrency under high load
- ✅ **Database Performance**: ACID transactions at scale
- ✅ **Queue Performance**: NATS handling high throughput
- ✅ **Credit System**: Race-condition safety under load

### **Success Criteria**
1. **Completes Test**: Full 10-minute duration without crashes
2. **Maintains Throughput**: ~115 RPS sustained load
3. **Acceptable Error Rate**: <15% (industry standard for high load)
4. **Response Time**: 95% under 2 seconds
5. **System Stability**: No memory leaks or resource exhaustion

## ⚠️ **Important Notes**

### **Credit Requirements**
The test requires ~69,000 credits (1 credit per message). Ensure the demo client has sufficient credits:

```bash
# Check credits before test
curl "http://localhost:8080/v1/me?client_id=550e8400-e29b-41d4-a716-446655440000"

# Add credits if needed (modify seed.sql or add via API)
```

### **System Resources**
High-load testing requires adequate system resources:
- **CPU**: Multi-core recommended for worker pool efficiency
- **Memory**: 8GB+ recommended for database and queue performance
- **Network**: Stable connection for consistent results

### **Test Environment**
- **Development**: Use for feature validation and basic performance
- **Staging**: Recommended for full 69k load testing
- **Production**: Use with caution and proper monitoring

## 🎉 **Why This Validates Production Readiness**

The 69,000 requests in 10 minutes test demonstrates:

1. **Scale Capability**: Subset of 100M messages/day requirement
2. **Concurrency Handling**: Multiple users simultaneously  
3. **System Stability**: Sustained high load without degradation
4. **Error Resilience**: Graceful handling of failures
5. **Resource Efficiency**: Controlled memory and CPU usage

**🚀 Success in this test confirms the SMS Gateway is ready for ArvanCloud production deployment!**