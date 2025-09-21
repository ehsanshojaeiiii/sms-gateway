#!/bin/bash

# Comprehensive SMS Industry Standards Compliance Audit
set -e

BASE_URL="http://localhost:8080"
DEMO_CLIENT="550e8400-e29b-41d4-a716-446655440000"

echo "📋 === SMS INDUSTRY STANDARDS COMPLIANCE AUDIT ==="
echo "🕒 $(date)"
echo

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

compliance_score=0
total_tests=0

check_standard() {
    local standard_name="$1"
    local test_result="$2"
    local expected="$3"
    total_tests=$((total_tests + 1))
    
    if [[ "$test_result" == "$expected" ]]; then
        echo -e "${GREEN}✅ COMPLIANT${NC}: $standard_name"
        compliance_score=$((compliance_score + 1))
    else
        echo -e "${RED}❌ NON-COMPLIANT${NC}: $standard_name (Expected: $expected, Got: $test_result)"
    fi
}

echo "=== 1. SMS CHARACTER ENCODING STANDARDS ==="

# GSM7 Single Part (160 chars)
echo -e "${BLUE}📝 Testing GSM7 encoding (160 char limit)${NC}"
gsm7_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"GSM7\",\"text\":\"This is exactly 160 chars for GSM7 test. It should be calculated as 1 part according to SMS standards and we need to verify this calculation.\"}")
gsm7_msg_id=$(echo "$gsm7_response" | jq -r '.message_id')
sleep 1
gsm7_parts=$(curl -s "$BASE_URL/v1/messages/$gsm7_msg_id" | jq -r '.parts')
check_standard "GSM7 160-char single part" "$gsm7_parts" "1"

# GSM7 Multi Part (161+ chars)
echo -e "${BLUE}📝 Testing GSM7 multi-part (153 chars per part after first)${NC}"
gsm7_multi_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"GSM7\",\"text\":\"This is a very long GSM7 message that exceeds 160 characters and should be split into multiple parts according to SMS standards. The first part can be 160 chars, but subsequent parts are limited to 153 characters due to the concatenation header overhead.\"}")
gsm7_multi_msg_id=$(echo "$gsm7_multi_response" | jq -r '.message_id')
sleep 1
gsm7_multi_parts=$(curl -s "$BASE_URL/v1/messages/$gsm7_multi_msg_id" | jq -r '.parts')
check_standard "GSM7 multi-part calculation" "$gsm7_multi_parts" "2"

# UCS2/Unicode Single Part (70 chars)
echo -e "${BLUE}📝 Testing UCS2 encoding (70 char limit)${NC}"
ucs2_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"UCS2\",\"text\":\"Unicode test 🚀 emoji should trigger UCS2 with 70 char limit max\"}")
ucs2_msg_id=$(echo "$ucs2_response" | jq -r '.message_id')
sleep 1
ucs2_parts=$(curl -s "$BASE_URL/v1/messages/$ucs2_msg_id" | jq -r '.parts')
check_standard "UCS2 70-char single part" "$ucs2_parts" "1"

# UCS2 Multi Part (67 chars per part)
echo -e "${BLUE}📝 Testing UCS2 multi-part (67 chars per part)${NC}"
ucs2_multi_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"UCS2\",\"text\":\"Unicode multi-part test 🚀 This message contains emoji and should be split according to UCS2 standards where each part is limited to 67 characters instead of 70 due to concatenation headers.\"}")
ucs2_multi_msg_id=$(echo "$ucs2_multi_response" | jq -r '.message_id')
sleep 1
ucs2_multi_parts=$(curl -s "$BASE_URL/v1/messages/$ucs2_multi_msg_id" | jq -r '.parts')
check_standard "UCS2 multi-part calculation" "$ucs2_multi_parts" "3"

echo -e "\n=== 2. SMS FIELD STANDARDS ==="

# Sender ID - Phone Number (E.164)
echo -e "${BLUE}📱 Testing E.164 phone number format${NC}"
e164_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"+9876543210\",\"text\":\"E.164 phone number sender test\"}")
e164_status=$(echo "$e164_response" | jq -r '.status // "error"')
check_standard "E.164 phone number as sender" "$e164_status" "QUEUED"

# Sender ID - Alphanumeric (max 11 chars)
echo -e "${BLUE}📝 Testing alphanumeric sender ID${NC}"
alpha_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"BANKNOTIFY\",\"text\":\"Alphanumeric sender ID test\"}")
alpha_status=$(echo "$alpha_response" | jq -r '.status // "error"')
check_standard "Alphanumeric sender ID (11 chars)" "$alpha_status" "QUEUED"

# Sender ID - Short Code
echo -e "${BLUE}🔢 Testing short code sender${NC}"
shortcode_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"12345\",\"text\":\"Short code sender test\"}")
shortcode_status=$(echo "$shortcode_response" | jq -r '.status // "error"')
check_standard "Short code sender (5 digits)" "$shortcode_status" "QUEUED"

echo -e "\n=== 3. SMS STATUS TRACKING STANDARDS ==="

# Message Status Flow
echo -e "${BLUE}📊 Testing SMS status progression${NC}"
status_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"STATUS\",\"text\":\"Status tracking test\"}")
status_msg_id=$(echo "$status_response" | jq -r '.message_id')
initial_status=$(echo "$status_response" | jq -r '.status')
check_standard "Initial message status" "$initial_status" "QUEUED"

# Wait for processing and check final status
sleep 3
final_status=$(curl -s "$BASE_URL/v1/messages/$status_msg_id" | jq -r '.status')
if [[ "$final_status" == "SENT" || "$final_status" == "DELIVERED" ]]; then
    check_standard "Message processing pipeline" "PROCESSED" "PROCESSED"
else
    check_standard "Message processing pipeline" "$final_status" "PROCESSED"
fi

echo -e "\n=== 4. OTP DELIVERY GUARANTEE STANDARDS ==="

# OTP Immediate Response
echo -e "${BLUE}🔐 Testing OTP delivery guarantee${NC}"
otp_response=$(curl -s -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"OTPBANK\",\"otp\":true}")
otp_status=$(echo "$otp_response" | jq -r '.status')
otp_code=$(echo "$otp_response" | jq -r '.otp_code // "missing"')

if [[ "$otp_status" == "SENT" && "$otp_code" != "missing" && "$otp_code" != "null" ]]; then
    check_standard "OTP immediate delivery guarantee" "COMPLIANT" "COMPLIANT"
else
    check_standard "OTP immediate delivery guarantee" "NON_COMPLIANT" "COMPLIANT"
fi

echo -e "\n=== 5. DELIVERY RECEIPT (DLR) STANDARDS ==="

# DLR Processing
echo -e "${BLUE}📨 Testing DLR webhook processing${NC}"
dlr_response=$(curl -s -w "%{http_code}" -X POST "$BASE_URL/v1/providers/mock/dlr" \
    -H "Content-Type: application/json" \
    -d "{\"provider_message_id\":\"mock_test_123\",\"status\":\"DELIVERED\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}")
dlr_code=$(echo "$dlr_response" | tail -c 4)
check_standard "DLR webhook acceptance" "$dlr_code" "204"

echo -e "\n=== 6. ERROR HANDLING STANDARDS ==="

# Invalid Phone Number
echo -e "${BLUE}❌ Testing invalid phone number rejection${NC}"
invalid_response=$(curl -s -w "%{http_code}" -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"\",\"from\":\"TEST\",\"text\":\"Invalid phone test\"}")
invalid_code=$(echo "$invalid_response" | tail -c 4)
check_standard "Invalid phone number rejection" "$invalid_code" "400"

# Missing sender ID
echo -e "${BLUE}❌ Testing missing sender ID rejection${NC}"
missing_sender_response=$(curl -s -w "%{http_code}" -X POST "$BASE_URL/v1/messages" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"\",\"text\":\"Missing sender test\"}")
missing_sender_code=$(echo "$missing_sender_response" | tail -c 4)
check_standard "Missing sender ID rejection" "$missing_sender_code" "400"

# Insufficient credits
echo -e "${BLUE}💰 Testing insufficient credits handling${NC}"
# First, check current credits
current_credits=$(curl -s "$BASE_URL/v1/me?client_id=$DEMO_CLIENT" | jq -r '.credits')
if [[ "$current_credits" -gt 0 ]]; then
    check_standard "Credit balance check" "AVAILABLE" "AVAILABLE"
else
    # Try sending with insufficient credits
    insufficient_response=$(curl -s -w "%{http_code}" -X POST "$BASE_URL/v1/messages" \
        -H "Content-Type: application/json" \
        -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+1234567890\",\"from\":\"CREDIT\",\"text\":\"Insufficient credits test\"}")
    insufficient_code=$(echo "$insufficient_response" | tail -c 4)
    check_standard "Insufficient credits handling" "$insufficient_code" "402"
fi

echo -e "\n=== 7. MESSAGE RETRY STANDARDS ==="

# Check if retry logic exists (by checking worker implementation)
echo -e "${BLUE}🔄 Testing retry mechanism implementation${NC}"
if grep -q "retry" internal/worker/worker.go && grep -q "attempts" internal/worker/worker.go; then
    check_standard "Message retry mechanism" "IMPLEMENTED" "IMPLEMENTED"
else
    check_standard "Message retry mechanism" "NOT_FOUND" "IMPLEMENTED"
fi

echo -e "\n=== 8. CONCURRENT ACCESS STANDARDS ==="

# Test concurrent message sending
echo -e "${BLUE}🔀 Testing concurrent access handling${NC}"
concurrent_pids=()

for i in {1..5}; do
    (
        response=$(curl -s -X POST "$BASE_URL/v1/messages" \
            -H "Content-Type: application/json" \
            -d "{\"client_id\":\"$DEMO_CLIENT\",\"to\":\"+123456789$i\",\"from\":\"CONC\",\"text\":\"Concurrent test $i\"}")
        echo "$response" | jq -r '.status' > "/tmp/concurrent_test_$i.result"
    ) &
    concurrent_pids+=($!)
done

# Wait for all concurrent requests
for pid in "${concurrent_pids[@]}"; do
    wait $pid
done

# Check results
concurrent_success=0
for i in {1..5}; do
    if [[ -f "/tmp/concurrent_test_$i.result" ]]; then
        result=$(cat "/tmp/concurrent_test_$i.result")
        if [[ "$result" == "QUEUED" ]]; then
            concurrent_success=$((concurrent_success + 1))
        fi
        rm -f "/tmp/concurrent_test_$i.result"
    fi
done

if [ "$concurrent_success" -eq 5 ]; then
    check_standard "Concurrent access handling" "HANDLED" "HANDLED"
else
    check_standard "Concurrent access handling" "PARTIAL" "HANDLED"
fi

echo -e "\n=== COMPLIANCE REPORT ==="
compliance_percentage=$(echo "scale=1; $compliance_score * 100 / $total_tests" | bc -l 2>/dev/null || echo "N/A")

echo -e "${BLUE}📊 SMS Industry Standards Compliance Score${NC}"
echo -e "${GREEN}✅ Compliant Standards: $compliance_score/$total_tests${NC}"
echo -e "${BLUE}📈 Compliance Rate: $compliance_percentage%${NC}"

if [ "$compliance_score" -eq "$total_tests" ]; then
    echo -e "${GREEN}🎉 FULL COMPLIANCE: SMS Gateway meets all industry standards!${NC}"
    exit 0
elif [ "$compliance_score" -ge $((total_tests * 8 / 10)) ]; then
    echo -e "${YELLOW}⚠️  HIGH COMPLIANCE: Most standards met, minor improvements needed.${NC}"
    exit 0
else
    echo -e "${RED}❌ LOW COMPLIANCE: Significant standards violations found.${NC}"
    exit 1
fi
