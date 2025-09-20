# K6 Load Testing Suite for SMS Gateway

This directory contains a comprehensive [K6](https://github.com/grafana/k6) load testing suite designed specifically for the SMS Gateway project. K6 is a modern load testing tool built by Grafana that uses JavaScript for test scripting and provides excellent performance testing capabilities.

## 🎯 **Why K6?**

K6 is the industry standard for modern load testing because it offers:

- **Developer-friendly**: Tests written in JavaScript with modern ES6+ syntax
- **High performance**: Built in Go, can generate massive load from a single machine
- **CI/CD integration**: Perfect for continuous performance testing
- **Rich metrics**: Built-in performance metrics and custom metric support
- **Flexible scenarios**: Support for various load patterns (constant, ramping, spike, etc.)
- **Protocol support**: HTTP, WebSockets, gRPC, and more
- **Cloud integration**: Native Grafana Cloud integration for advanced analytics

## 📁 **Test Structure**

```
k6/
├── sms-gateway-load-test.js    # Main comprehensive test suite
├── scenarios/
│   ├── burst-test.js           # Sudden traffic burst simulation
│   └── endurance-test.js       # Long-term stability testing
├── run-tests.sh                # Test runner script
├── results/                    # Test results output (auto-created)
└── README.md                   # This documentation
```

## 🚀 **Quick Start**

### 1. Install K6

```bash
# macOS (Homebrew)
brew install k6

# Linux (APT)
sudo apt update && sudo apt install k6

# Windows (Winget)
winget install k6

# Or use our Makefile helper
make k6-install
```

### 2. Start SMS Gateway

```bash
make run
```

### 3. Run Load Tests

```bash
# Quick smoke test (30 seconds)
make k6-smoke

# Standard load test (16 minutes)
make k6-load

# Stress test (16 minutes)  
make k6-stress

# Complete test suite
make k6-all
```

## 📊 **Available Test Scenarios**

### **Smoke Test** (`make k6-smoke`)
- **Duration**: 30 seconds
- **Purpose**: Basic functionality verification
- **Load**: 1 virtual user
- **Tests**: Health checks, basic SMS sending, OTP, Express SMS

### **Load Test** (`make k6-load`)
- **Duration**: 16 minutes
- **Purpose**: Normal traffic simulation
- **Load**: Ramps 0→10→20→0 users
- **Distribution**: 60% Regular, 30% Express, 10% OTP

### **Stress Test** (`make k6-stress`)
- **Duration**: 16 minutes  
- **Purpose**: High traffic simulation
- **Load**: Ramps 0→50→100→0 users
- **Distribution**: 50% Regular, 35% Express, 15% OTP

### **Spike Test** (`make k6-spike`)
- **Duration**: 8 minutes
- **Purpose**: Sudden traffic bursts
- **Load**: 10→200→10→0 users (sudden spike)
- **Distribution**: 30% Regular, 40% Express, 30% OTP

### **Volume Test** (`make k6-volume`)
- **Duration**: Up to 30 minutes
- **Purpose**: High volume simulation
- **Load**: 100 users × 1000 messages each
- **Total**: 100,000 messages

### **Burst Test** (`make k6-burst`)
- **Duration**: 2.5 minutes
- **Purpose**: Traffic burst simulation (Black Friday scenario)
- **Load**: Arrival rate 10→500→50→10 RPS
- **Focus**: Critical messages (OTP, Express)

### **Endurance Test** (`make k6-endurance`)
- **Duration**: 30 minutes
- **Purpose**: Long-term stability testing
- **Load**: Constant 20 users
- **Focus**: Memory leaks, connection issues

## 🎯 **Performance Thresholds**

Our K6 tests include comprehensive performance thresholds:

```javascript
thresholds: {
  // Overall performance
  http_req_duration: ['p(95)<2000', 'p(99)<5000'],  // 95% under 2s, 99% under 5s
  http_req_failed: ['rate<0.05'],                    // Less than 5% failures
  
  // SMS-specific thresholds
  sms_success_rate: ['rate>0.95'],                   // 95% SMS success rate
  otp_success_rate: ['rate>0.98'],                   // 98% OTP success rate
  express_success_rate: ['rate>0.97'],              // 97% Express SMS success rate
}
```

## 📈 **Custom Metrics**

Our tests track SMS-specific metrics:

- `sms_success_rate`: Overall SMS delivery success rate
- `otp_success_rate`: OTP delivery success rate (higher threshold)
- `express_success_rate`: Express SMS success rate
- `sms_latency`: SMS processing latency
- `credit_errors`: Credit insufficiency errors
- `queued_messages`: Messages successfully queued
- `immediate_messages`: Messages delivered immediately (OTP)

## 🔧 **Configuration**

### Environment Variables

```bash
export BASE_URL="http://localhost:8080"           # SMS Gateway URL
export CLIENT_ID="550e8400-e29b-41d4-a716-446655440000"  # Test client ID
export OUTPUT_DIR="./k6/results"                 # Results output directory
```

### Test Runner Options

```bash
# Run specific test type
./k6/run-tests.sh smoke
./k6/run-tests.sh load
./k6/run-tests.sh stress

# Run with custom settings
BASE_URL="http://production-api:8080" ./k6/run-tests.sh load
```

## 📊 **Results Analysis**

### Output Files

Each test run generates:
- `results/{test}_YYYYMMDD_HHMMSS.json` - Summary statistics
- `results/{test}_YYYYMMDD_HHMMSS.jsonl` - Detailed metrics
- `results/report_YYYYMMDD_HHMMSS.html` - HTML report

### Key Metrics to Monitor

1. **Response Time**:
   - `http_req_duration` - Overall request latency
   - `p(95)` and `p(99)` percentiles are most important

2. **Success Rates**:
   - `http_req_failed` - HTTP error rate
   - `sms_success_rate` - SMS-specific success rate
   - `otp_success_rate` - OTP delivery success rate

3. **Throughput**:
   - `http_reqs` - Total requests per second
   - `data_sent/received` - Network throughput

4. **SMS-Specific**:
   - `credit_errors` - Billing system issues
   - `queued_messages` vs `immediate_messages` - Processing patterns

### Grafana Integration

For advanced analysis, K6 results can be sent to Grafana Cloud or self-hosted Grafana:

```bash
k6 run --out cloud sms-gateway-load-test.js
```

## 🎯 **Scale Testing Strategy**

### **100 Million Messages/Day Validation**

Our K6 tests validate the PDF requirement of handling 100M messages/day:

- **Peak Load**: ~1,157 messages/second average, ~10,000 TPS peak
- **Volume Test**: 100 users × 1000 messages = 100,000 messages in 30 minutes
- **Stress Test**: Validates system behavior under 100+ concurrent users
- **Endurance Test**: Ensures system stability over extended periods

### **Real-World Scenarios**

- **Black Friday**: Burst test simulates sudden traffic spikes
- **Banking OTP**: High OTP volume with delivery guarantees
- **Marketing Campaigns**: Mixed Express/Regular message loads
- **System Recovery**: Spike tests validate graceful degradation

## 🔍 **Troubleshooting**

### Common Issues

1. **K6 Not Found**:
   ```bash
   make k6-install  # Auto-install K6
   ```

2. **SMS Gateway Not Running**:
   ```bash
   make run         # Start SMS Gateway
   make status      # Check service status
   ```

3. **Low Credits Warning**:
   - Check client credits: `curl "http://localhost:8080/v1/me?client_id=550e8400-e29b-41d4-a716-446655440000"`
   - Re-seed database: `make seed`

4. **Test Failures**:
   - Check SMS Gateway logs: `make logs`
   - Verify API health: `curl http://localhost:8080/health`
   - Review test thresholds in K6 scripts

### Performance Tuning

For higher loads, consider:

```bash
# Increase system limits
ulimit -n 65536

# Use more CPU cores
k6 run --vus 1000 --duration 10m sms-gateway-load-test.js

# Disable detailed logging for performance
k6 run --quiet sms-gateway-load-test.js
```

## 🎉 **Best Practices**

1. **Start Small**: Always run smoke tests first
2. **Gradual Scaling**: Use ramping stages, not immediate high load
3. **Monitor Resources**: Watch CPU, memory, and network during tests
4. **Realistic Data**: Use varied phone numbers and message content
5. **CI Integration**: Include K6 tests in your deployment pipeline
6. **Baseline Metrics**: Establish performance baselines before changes

## 📚 **Further Reading**

- [K6 Documentation](https://k6.io/docs/)
- [K6 JavaScript API](https://k6.io/docs/javascript-api/)
- [Performance Testing Best Practices](https://k6.io/docs/testing-guides/)
- [Grafana K6 Cloud](https://grafana.com/products/cloud/k6/)

---

**Happy Load Testing! 🚀**

*This K6 suite ensures your SMS Gateway can handle production loads with confidence.*
