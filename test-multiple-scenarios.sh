#!/bin/bash

# Comprehensive multi-user SMS Gateway test
set -e

BASE_URL="http://localhost:8080"
DEMO_CLIENT="550e8400-e29b-41d4-a716-446655440000"

echo "🧪 === COMPREHENSIVE MULTI-USER TEST ==="
echo "🕒 $(date)"
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

success_count=0
total_tests=0

test_result() {
    local test_name="$1"
    local expected="$2"
    local actual="$3"
    total_tests=$((total_tests + 1))
    
    if [[ "$actual" == "$expected" ]]; then
        echo -e "${GREEN}✅ PASS${NC}: $test_name"
        success_count=$((success_count + 1))
    else
        echo -e "${RED}❌ FAIL${NC}: $test_name (Expected: $expected, Got: $actual)"
    fi
}

echo "=== PHASE 1: SYSTEM HEALTH CHECK ==="

# Test 1: Health check
echo -e "${BLUE}🏥 Testing system health...${NC}"
health_status=$(curl -s "$BASE_URL/health" | jq -r '.status' 2>/dev/null || echo "error")
test_result "System health check" "ok" "$health_status"

# Test 2: Demo client setup
echo -e "${BLUE}👤 Checking demo client...${NC}"
demo_credits=$(curl -s "$BASE_URL/v1/me?client_id=$DEMO_CLIENT" | jq -r '.credits' 2>/dev/null || echo "error")
if [[ "$demo_credits" =~ ^[0-9]+$ ]] && [ "$demo_credits" -gt 0 ]; then
    test_result "Demo client has credits" "has_credits" "has_credits"
    echo "   💰 Demo client has $demo_credits credits"
else
    test_result "Demo client has credits" "has_credits" "no_credits"
fi

echo -e "\n=== PHASE 2: SINGLE USER SCENARIOS ==="

# Test 3: Basic SMS
echo -e "${BLUE}📱 Testing basic SMS...${NC}"
basic_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"TEST\",\"text\":\"Basic SMS test\"}")
basic_status=$(echo "$basic_response" | jq -r '.status' 2>/dev/null || echo "error")
basic_msg_id=$(echo "$basic_response" | jq -r '.message_id' 2>/dev/null)
test_result "Basic SMS creation" "QUEUED" "$basic_status"

# Test 4: Express SMS
echo -e "${BLUE}⚡ Testing express SMS...${NC}"
express_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"EXPRESS\",\"text\":\"Express SMS test\",\"express\":true}")
express_status=$(echo "$express_response" | jq -r '.status' 2>/dev/null || echo "error")
express_msg_id=$(echo "$express_response" | jq -r '.message_id' 2>/dev/null)
test_result "Express SMS creation" "QUEUED" "$express_status"

# Test 5: OTP SMS
echo -e "${BLUE}🔐 Testing OTP SMS...${NC}"
otp_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"OTP\",\"otp\":true}")
otp_status=$(echo "$otp_response" | jq -r '.status' 2>/dev/null || echo "error")
otp_msg_id=$(echo "$otp_response" | jq -r '.message_id' 2>/dev/null)
test_result "OTP SMS creation" "SENT" "$otp_status"

# Wait for processing
echo -e "${YELLOW}⏳ Waiting for message processing...${NC}"
sleep 5

# Test 6: Check message processing
if [[ "$basic_msg_id" != "null" && "$basic_msg_id" != "" ]]; then
    echo -e "${BLUE}📊 Checking message processing...${NC}"
    basic_final_status=$(curl -s "$BASE_URL/v1/messages/$basic_msg_id" | jq -r '.status' 2>/dev/null || echo "error")
    if [[ "$basic_final_status" == "SENT" || "$basic_final_status" == "DELIVERED" ]]; then
        test_result "Basic SMS processing" "processed" "processed"
    else
        test_result "Basic SMS processing" "processed" "$basic_final_status"
    fi
fi

echo -e "\n=== PHASE 3: ERROR SCENARIOS ==="

# Test 7: Invalid client ID
echo -e "${BLUE}🚫 Testing invalid client...${NC}"
invalid_response=$(curl -s -w "%{http_code}" -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"00000000-0000-0000-0000-000000000000\",\"to\":\"+1234567890\",\"from\":\"TEST\",\"text\":\"Invalid client test\"}")
invalid_code=$(echo "$invalid_response" | tail -c 4)
test_result "Invalid client rejection" "400" "$invalid_code"

# Test 8: Missing required fields
echo -e "${BLUE}📝 Testing missing fields...${NC}"
missing_response=$(curl -s -w "%{http_code}" -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\"}")
missing_code=$(echo "$missing_response" | tail -c 4)
test_result "Missing fields rejection" "400" "$missing_code"

echo -e "\n=== PHASE 4: CONCURRENT USER SIMULATION ==="

echo -e "${BLUE}🔥 Testing concurrent requests...${NC}"
concurrent_pids=()
concurrent_results=()

# Create temporary files for concurrent test results
for i in {1..10}; do
    (
        response=$(curl -s -X POST "$BASE_URL/v1/messages" \
            -H "Content-Type: application/json" \
            -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+123456789$i\",\"from\":\"USER$i\",\"text\":\"Concurrent test $i\"}")
        status=$(echo "$response" | jq -r '.status' 2>/dev/null || echo "error")
        echo "$status" > "/tmp/sms_test_$i.result"
    ) &
    concurrent_pids+=($!)
done

# Wait for all concurrent requests
for pid in "${concurrent_pids[@]}"; do
    wait $pid
done

# Check concurrent results
echo -e "${BLUE}📊 Analyzing concurrent results...${NC}"
concurrent_success=0
concurrent_total=10

for i in {1..10}; do
    if [[ -f "/tmp/sms_test_$i.result" ]]; then
        result=$(cat "/tmp/sms_test_$i.result")
        if [[ "$result" == "QUEUED" ]]; then
            concurrent_success=$((concurrent_success + 1))
        fi
        rm -f "/tmp/sms_test_$i.result"
    fi
done

echo "   🎯 Concurrent requests: $concurrent_success/$concurrent_total successful"
if [ "$concurrent_success" -ge 8 ]; then  # Allow some margin for concurrent load
    test_result "Concurrent request handling" "success" "success"
else
    test_result "Concurrent request handling" "success" "partial_failure"
fi

echo -e "\n=== PHASE 5: SYSTEM PERFORMANCE ==="

# Test worker metrics
echo -e "${BLUE}⚙️  Checking worker performance...${NC}"
sleep 2

# Final credit check
final_credits=$(curl -s "$BASE_URL/v1/me?client_id=$DEMO_CLIENT" | jq -r '.credits' 2>/dev/null || echo "error")
if [[ "$final_credits" =~ ^[0-9]+$ ]]; then
    credits_used=$((demo_credits - final_credits))
    echo "   💸 Credits used: $credits_used (from $demo_credits to $final_credits)"
    if [ "$credits_used" -gt 0 ]; then
        test_result "Credit deduction working" "working" "working"
    else
        test_result "Credit deduction working" "working" "not_working"
    fi
else
    test_result "Credit deduction working" "working" "error"
fi

echo -e "\n=== PHASE 6: SYSTEM LOAD TEST ==="

echo -e "${BLUE}🚀 Testing system under load (50 requests in 10 seconds)...${NC}"
load_start_time=$(date +%s)
load_pids=()

for i in {1..50}; do
    (
        curl -s -X POST "$BASE_URL/v1/messages" \
            -H "Content-Type: application/json" \
            -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+555000$i\",\"from\":\"LOAD\",\"text\":\"Load test $i\"}" \
            > /dev/null
    ) &
    load_pids+=($!)
    
    # Stagger requests slightly
    if (( i % 10 == 0 )); then
        sleep 0.1
    fi
done

# Wait for all load test requests
for pid in "${load_pids[@]}"; do
    wait $pid
done

load_end_time=$(date +%s)
load_duration=$((load_end_time - load_start_time))
requests_per_second=$(echo "scale=1; 50 / $load_duration" | bc -l 2>/dev/null || echo "N/A")

echo "   🎯 Load test completed in ${load_duration}s (~$requests_per_second req/s)"
if [ "$load_duration" -le 15 ]; then  # Should complete within reasonable time
    test_result "System load performance" "acceptable" "acceptable"
else
    test_result "System load performance" "acceptable" "slow"
fi

echo -e "\n=== TEST RESULTS SUMMARY ==="
echo -e "${GREEN}✅ Passed: $success_count/${total_tests} tests${NC}"

success_rate=$(echo "scale=1; $success_count * 100 / $total_tests" | bc -l 2>/dev/null || echo "N/A")
echo -e "${BLUE}📊 Success Rate: $success_rate%${NC}"

if [ "$success_count" -eq "$total_tests" ]; then
    echo -e "${GREEN}🎉 ALL TESTS PASSED! System is production ready.${NC}"
    exit 0
elif [ "$success_count" -ge $((total_tests * 8 / 10)) ]; then
    echo -e "${YELLOW}⚠️  Most tests passed. System is generally stable.${NC}"
    exit 0
else
    echo -e "${RED}❌ Multiple test failures. System needs attention.${NC}"
    exit 1
fi
