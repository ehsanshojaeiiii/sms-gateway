#!/bin/bash

# SMS Gateway K6 Load Testing Suite
# This script runs comprehensive load tests for the SMS Gateway

set -e

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
CLIENT_ID="${CLIENT_ID:-550e8400-e29b-41d4-a716-446655440000}"
OUTPUT_DIR="${OUTPUT_DIR:-./results}"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo -e "${BLUE}🚀 SMS Gateway K6 Load Testing Suite${NC}"
echo -e "${BLUE}=====================================${NC}"
echo "📊 Base URL: $BASE_URL"
echo "🆔 Client ID: $CLIENT_ID"
echo "📁 Output Directory: $OUTPUT_DIR"
echo "⏰ Timestamp: $TIMESTAMP"
echo ""

# Function to check if system is ready
check_system() {
    echo -e "${YELLOW}🔍 Checking system readiness...${NC}"
    
    # Check if API is responding
    if ! curl -s "$BASE_URL/health" > /dev/null; then
        echo -e "${RED}❌ System not ready. Please start the SMS Gateway first.${NC}"
        echo "   Run: make run"
        exit 1
    fi
    
    # Check client credits
    CREDITS=$(curl -s "$BASE_URL/v1/me?client_id=$CLIENT_ID" | jq -r '.credits // 0' 2>/dev/null || echo "0")
    echo "💰 Client Credits: $CREDITS"
    
    if [ "$CREDITS" -lt 10000 ]; then
        echo -e "${YELLOW}⚠️  Warning: Low credits ($CREDITS). Tests may fail.${NC}"
    fi
    
    echo -e "${GREEN}✅ System ready${NC}"
    echo ""
}

# Function to run a specific test
run_test() {
    local test_name=$1
    local test_file=$2
    local description=$3
    
    echo -e "${BLUE}🧪 Running $test_name${NC}"
    echo "📝 Description: $description"
    echo "📄 Test file: $test_file"
    
    local output_file="$OUTPUT_DIR/${test_name}_${TIMESTAMP}"
    
    # Run K6 test with comprehensive output
    if k6 run \
        --env BASE_URL="$BASE_URL" \
        --env CLIENT_ID="$CLIENT_ID" \
        --summary-export="$output_file.json" \
        --out json="$output_file.jsonl" \
        "$test_file"; then
        echo -e "${GREEN}✅ $test_name completed successfully${NC}"
    else
        echo -e "${RED}❌ $test_name failed${NC}"
        return 1
    fi
    
    echo ""
}

# Function to generate HTML report
generate_report() {
    echo -e "${BLUE}📊 Generating HTML Report...${NC}"
    
    # Create a simple HTML report (you could use k6-reporter or similar tools)
    cat > "$OUTPUT_DIR/report_${TIMESTAMP}.html" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>SMS Gateway Load Test Report - $TIMESTAMP</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f0f8ff; padding: 20px; border-radius: 8px; }
        .test-section { margin: 20px 0; padding: 15px; border: 1px solid #ddd; }
        .success { color: green; }
        .warning { color: orange; }
        .error { color: red; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 SMS Gateway Load Test Report</h1>
        <p><strong>Timestamp:</strong> $TIMESTAMP</p>
        <p><strong>Base URL:</strong> $BASE_URL</p>
        <p><strong>Client ID:</strong> $CLIENT_ID</p>
    </div>
    
    <div class="test-section">
        <h2>📋 Test Summary</h2>
        <p>Load test results are stored in JSON format in the results directory.</p>
        <p>Use K6 HTML reporter or Grafana for detailed visualization.</p>
    </div>
    
    <div class="test-section">
        <h2>📁 Output Files</h2>
        <ul>
$(find "$OUTPUT_DIR" -name "*_${TIMESTAMP}*" -type f | sed 's|^|            <li>|; s|$|</li>|')
        </ul>
    </div>
</body>
</html>
EOF

    echo -e "${GREEN}✅ HTML report generated: $OUTPUT_DIR/report_${TIMESTAMP}.html${NC}"
}

# Main execution
main() {
    check_system
    
    # Test selection based on command line argument
    case "${1:-all}" in
        "smoke")
            run_test "smoke-test" "sms-gateway-load-test.js" "Basic functionality verification"
            ;;
        "load")
            run_test "load-test" "sms-gateway-load-test.js" "Normal traffic simulation"
            ;;
        "stress")
            run_test "stress-test" "sms-gateway-load-test.js" "High traffic simulation"
            ;;
        "spike")
            run_test "spike-test" "sms-gateway-load-test.js" "Traffic spike simulation"
            ;;
        "volume")
            run_test "volume-test" "sms-gateway-load-test.js" "100 clients × 1000 messages"
            ;;
        "burst")
            run_test "burst-test" "scenarios/burst-test.js" "Sudden traffic burst simulation"
            ;;
        "endurance")
            run_test "endurance-test" "scenarios/endurance-test.js" "30-minute stability test"
            ;;
        "all")
            echo -e "${YELLOW}🎯 Running complete test suite...${NC}"
            run_test "smoke-test" "sms-gateway-load-test.js" "Basic functionality verification"
            run_test "load-test" "sms-gateway-load-test.js" "Normal traffic simulation"
            run_test "stress-test" "sms-gateway-load-test.js" "High traffic simulation"
            run_test "spike-test" "sms-gateway-load-test.js" "Traffic spike simulation"
            run_test "burst-test" "scenarios/burst-test.js" "Sudden traffic burst simulation"
            echo -e "${GREEN}🎉 Complete test suite finished${NC}"
            ;;
        "help"|*)
            echo "Usage: $0 [test_type]"
            echo ""
            echo "Available test types:"
            echo "  smoke     - Basic functionality verification (30s)"
            echo "  load      - Normal traffic simulation (16m)"
            echo "  stress    - High traffic simulation (16m)"
            echo "  spike     - Traffic spike simulation (8m)"
            echo "  volume    - 100 clients × 1000 messages (30m)"
            echo "  burst     - Sudden traffic burst (2.5m)"
            echo "  endurance - Long-term stability test (30m)"
            echo "  all       - Run smoke, load, stress, spike, burst tests"
            echo "  help      - Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 smoke          # Quick smoke test"
            echo "  $0 load           # Standard load test"
            echo "  $0 all            # Complete test suite"
            echo ""
            exit 0
            ;;
    esac
    
    generate_report
    
    echo -e "${GREEN}🏁 Testing completed!${NC}"
    echo "📊 Results saved to: $OUTPUT_DIR"
    echo "📈 View HTML report: $OUTPUT_DIR/report_${TIMESTAMP}.html"
}

# Check dependencies
if ! command -v k6 &> /dev/null; then
    echo -e "${RED}❌ K6 is not installed${NC}"
    echo "Install K6: https://k6.io/docs/getting-started/installation/"
    echo ""
    echo "Quick install options:"
    echo "  macOS: brew install k6"
    echo "  Linux: sudo apt install k6"
    echo "  Windows: winget install k6"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo -e "${YELLOW}⚠️  jq not found. Installing via package manager recommended.${NC}"
fi

# Run main function
main "$@"
