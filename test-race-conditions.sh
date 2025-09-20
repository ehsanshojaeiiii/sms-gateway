#!/bin/bash
# 🧪 Race Condition & Double Spending Test
# Tests concurrent credit access to prevent double spending

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BASE_URL="http://localhost:8080"
CLIENT_ID="550e8400-e29b-41d4-a716-446655440000"

echo -e "${BLUE}🧪 Race Condition & Double Spending Test${NC}"
echo "================================================="

# Step 1: Setup test client with known credits
echo -e "\n${YELLOW}1. Setting up test client with limited credits${NC}"
docker-compose exec postgres psql -U postgres -d sms_gateway -c "
    UPDATE clients SET credit_cents = 100 WHERE id = '$CLIENT_ID';
    SELECT id, name, credit_cents FROM clients WHERE id = '$CLIENT_ID';
"

initial_credits=$(curl -s "$BASE_URL/v1/me?client_id=$CLIENT_ID" | jq -r '.credits')
echo "Initial credits: $initial_credits cents"

# Step 2: Test Race Condition - 20 concurrent requests with 5 cent cost each
# Should only allow 20 messages max (100 credits ÷ 5 cents = 20 messages)
echo -e "\n${YELLOW}2. Testing 20 concurrent requests (edge case)${NC}"
echo "Each message costs 5 cents. With 100 credits, only 20 should succeed."

concurrent_count=20
success_count=0
fail_count=0
insufficient_credit_count=0

echo "Sending $concurrent_count concurrent requests..."

# Send concurrent requests
for i in $(seq 1 $concurrent_count); do
    (
        response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/messages" \
            -H "Content-Type: application/json" \
            -d "{\"client_id\":\"$CLIENT_ID\",\"to\":\"+123456789$(printf %02d $i)\",\"from\":\"RACE\",\"text\":\"Race test #$i\"}")
        
        status_code=$(echo "$response" | tail -n 1)
        body=$(echo "$response" | sed '$d')
        
        if [ "$status_code" = "202" ]; then
            echo "success" > "/tmp/race_test_$i.result"
        elif [ "$status_code" = "402" ]; then
            echo "insufficient" > "/tmp/race_test_$i.result"
        else
            echo "error:$status_code" > "/tmp/race_test_$i.result"
        fi
    ) &
done

wait # Wait for all requests to complete

# Count results
for i in $(seq 1 $concurrent_count); do
    if [ -f "/tmp/race_test_$i.result" ]; then
        result=$(cat "/tmp/race_test_$i.result")
        case "$result" in
            "success") ((success_count++));;
            "insufficient") ((insufficient_credit_count++));;
            *) ((fail_count++));;
        esac
        rm -f "/tmp/race_test_$i.result"
    fi
done

echo -e "\n${BLUE}Results:${NC}"
echo "  Successful (202): $success_count"
echo "  Insufficient credits (402): $insufficient_credit_count" 
echo "  Other errors: $fail_count"
echo "  Total processed: $((success_count + insufficient_credit_count + fail_count))"

# Step 3: Check final credits
final_credits=$(curl -s "$BASE_URL/v1/me?client_id=$CLIENT_ID" | jq -r '.credits')
credits_used=$((initial_credits - final_credits))
expected_credits_used=$((success_count * 5))

echo -e "\n${YELLOW}3. Credit Usage Analysis${NC}"
echo "  Initial credits: $initial_credits cents"
echo "  Final credits: $final_credits cents"
echo "  Credits used: $credits_used cents"
echo "  Expected usage: $expected_credits_used cents (${success_count} messages × 5 cents)"

# Step 4: Race condition evaluation
echo -e "\n${BLUE}📊 Race Condition Analysis:${NC}"

if [ "$credits_used" -eq "$expected_credits_used" ]; then
    echo -e "${GREEN}✅ PASS: Credits used exactly match successful messages${NC}"
    echo -e "${GREEN}✅ PASS: No double spending detected${NC}"
else
    echo -e "${RED}❌ FAIL: Credit mismatch detected!${NC}"
    echo -e "${RED}   This indicates a race condition in credit management${NC}"
fi

# Only 20 messages should succeed (100 credits ÷ 5 cents)
expected_max_success=20
if [ "$success_count" -le "$expected_max_success" ]; then
    echo -e "${GREEN}✅ PASS: Prevented overspending (${success_count}/${expected_max_success} max)${NC}"
else
    echo -e "${RED}❌ FAIL: Overspending detected! (${success_count}/${expected_max_success} allowed)${NC}"
fi

if [ "$insufficient_credit_count" -gt 0 ]; then
    echo -e "${GREEN}✅ PASS: Properly rejected insufficient credit requests${NC}"
else
    if [ "$success_count" -eq "$expected_max_success" ]; then
        echo -e "${GREEN}✅ PASS: Perfect credit management (exactly 20 succeeded)${NC}"
    else
        echo -e "${YELLOW}⚠️  WARNING: Expected some insufficient credit rejections${NC}"
    fi
fi

# Step 5: Database consistency check
echo -e "\n${YELLOW}4. Database Consistency Check${NC}"
echo "Checking credit locks in database:"
docker-compose exec postgres psql -U postgres -d sms_gateway -c "
    SELECT state, COUNT(*) 
    FROM credit_locks 
    WHERE client_id = '$CLIENT_ID' 
    GROUP BY state 
    ORDER BY state;
"

# Summary
echo -e "\n${BLUE}=================================${NC}"
if [ "$credits_used" -eq "$expected_credits_used" ] && [ "$success_count" -le "$expected_max_success" ]; then
    echo -e "${GREEN}🎉 RACE CONDITION TEST PASSED${NC}"
    echo -e "${GREEN}✅ Double spending prevented${NC}"
    echo -e "${GREEN}✅ Credit consistency maintained${NC}"
else
    echo -e "${RED}⚠️  RACE CONDITION ISSUES DETECTED${NC}"
    echo -e "${RED}❌ Needs additional protection${NC}"
fi
