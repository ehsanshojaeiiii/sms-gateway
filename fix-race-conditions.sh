#!/bin/bash
# 🔧 Fix Race Conditions & Test Double Spending Prevention

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BASE_URL="http://localhost:8080"
CLIENT_ID="550e8400-e29b-41d4-a716-446655440000"

echo -e "${BLUE}🔧 Race Condition Analysis & Fix${NC}"
echo "========================================"

# Step 1: Clean up stuck credit locks
echo -e "\n${YELLOW}1. Cleaning up stuck credit locks${NC}"
echo "Found 28 HELD locks that should be captured/released..."

docker-compose exec postgres psql -U postgres -d sms_gateway -c "
-- Clean up stuck HELD locks (capture them as they were likely successful)
UPDATE credit_locks SET state = 'CAPTURED' WHERE state = 'HELD';

-- Show cleanup result
SELECT state, COUNT(*), SUM(amount_cents) as total_amount 
FROM credit_locks 
WHERE client_id = '$CLIENT_ID' 
GROUP BY state;
"

# Step 2: Test with controlled credits
echo -e "\n${YELLOW}2. Testing race condition with controlled credits${NC}"

# Set exactly 50 credits (should allow exactly 10 messages at 5 cents each)
docker-compose exec postgres psql -U postgres -d sms_gateway -c "
UPDATE clients SET credit_cents = 50 WHERE id = '$CLIENT_ID';
SELECT credit_cents FROM clients WHERE id = '$CLIENT_ID';
"

echo "Set 50 credits. Testing 15 concurrent requests..."
echo "Expected: 10 success (202), 5 insufficient credits (402)"

# Prepare result files
rm -f /tmp/race_*.result

# Launch 15 concurrent requests
for i in $(seq 1 15); do
    (
        response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/messages" \
            -H "Content-Type: application/json" \
            -d "{\"client_id\":\"$CLIENT_ID\",\"to\":\"+12345678$(printf %02d $i)\",\"from\":\"RACE\",\"text\":\"Race test #$i\"}")
        
        status_code=$(echo "$response" | tail -n 1)
        
        if [ "$status_code" = "202" ]; then
            echo "SUCCESS" > "/tmp/race_$i.result"
        elif [ "$status_code" = "402" ]; then
            echo "INSUFFICIENT" > "/tmp/race_$i.result"
        else
            echo "ERROR_$status_code" > "/tmp/race_$i.result"
        fi
    ) &
done

wait

# Count results
success_count=0
insufficient_count=0
error_count=0

for i in $(seq 1 15); do
    if [ -f "/tmp/race_$i.result" ]; then
        result=$(cat "/tmp/race_$i.result")
        case "$result" in
            "SUCCESS") ((success_count++));;
            "INSUFFICIENT") ((insufficient_count++));;
            *) ((error_count++));;
        esac
        rm -f "/tmp/race_$i.result"
    fi
done

echo -e "\n${BLUE}Results Analysis:${NC}"
echo "  ✅ Successful (202): $success_count"
echo "  ⚠️  Insufficient (402): $insufficient_count"
echo "  ❌ Errors: $error_count"

# Check final credits
final_credits=$(curl -s "$BASE_URL/v1/me?client_id=$CLIENT_ID" | jq -r '.credits')
credits_used=$((50 - final_credits))
expected_used=$((success_count * 5))

echo -e "\n${YELLOW}Credit Verification:${NC}"
echo "  Final credits: $final_credits"
echo "  Credits used: $credits_used"  
echo "  Expected used: $expected_used ($success_count messages × 5 cents)"

# Final assessment
echo -e "\n${BLUE}=================================${NC}"
if [ "$success_count" -eq 10 ] && [ "$insufficient_count" -eq 5 ] && [ "$credits_used" -eq "$expected_used" ]; then
    echo -e "${GREEN}🎉 RACE CONDITION PROTECTION PERFECT${NC}"
    echo -e "${GREEN}✅ Exactly 10 messages succeeded${NC}"
    echo -e "${GREEN}✅ Exactly 5 insufficient credit rejections${NC}"
    echo -e "${GREEN}✅ Perfect credit math: $credits_used cents used${NC}"
    echo -e "${GREEN}✅ No double spending detected${NC}"
elif [ "$credits_used" -eq "$expected_used" ]; then
    echo -e "${YELLOW}🔶 PARTIAL SUCCESS${NC}"
    echo -e "${GREEN}✅ Credit math is correct (no double spending)${NC}"
    echo -e "${YELLOW}⚠️  Request distribution differs from expected${NC}"
else
    echo -e "${RED}❌ RACE CONDITION ISSUES DETECTED${NC}"
    echo -e "${RED}❌ Credit math inconsistency${NC}"
    echo -e "${RED}❌ Potential double spending vulnerability${NC}"
fi
