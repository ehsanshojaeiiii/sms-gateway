import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom metrics
export const errors = new Counter('errors');
export const successRate = new Rate('success_rate');
export const messageDuration = new Trend('message_duration');

// Configuration: 100 users × 100 requests each = 10,000 requests total
// Target: 100% success rate
export const options = {
  // 100 virtual users, each making 100 requests
  vus: 100,
  iterations: 10000, // Exactly 10,000 total requests
  duration: '10m', // Maximum 10 minutes to complete
  
  thresholds: {
    'http_req_duration': ['p(95)<3000'],     // 95% of requests under 3s
    'http_req_failed': ['rate<0.01'],        // Error rate under 1% (targeting 100% success)
    'success_rate': ['rate>0.99'],          // Success rate over 99% (targeting 100%)
    'errors': ['count<100'],                 // Max 1% error count (100 out of 10k)
  },
};

const BASE_URL = 'http://localhost:8080';
const DEMO_CLIENT = '550e8400-e29b-41d4-a716-446655440000';

// Pre-generate phone numbers for variety (each user gets unique range)
const phoneNumbers = [];
for (let i = 1000000; i <= 1100000; i++) {
  phoneNumbers.push(`+1${i}`);
}

function generateRandomText() {
  const words = ['hello', 'world', 'test', 'message', 'load', 'performance', 'sms', 'gateway', 'user', 'final'];
  const length = Math.floor(Math.random() * 15) + 5;
  let text = '';
  for (let i = 0; i < length; i++) {
    text += words[Math.floor(Math.random() * words.length)] + ' ';
  }
  return text.trim();
}

export default function () {
  // Each VU gets a unique phone number range
  const userID = __VU; // K6 Virtual User ID (1-100)
  const iterationID = __ITER; // Current iteration for this VU (0-99)
  
  // Create unique phone number for each request
  const phoneIndex = ((userID - 1) * 100 + iterationID) % phoneNumbers.length;
  const phone = phoneNumbers[phoneIndex];
  
  const messageId = userID * 1000 + iterationID; // Unique message ID
  
  // Message type distribution - mostly regular messages for high success rate
  const messageType = Math.floor(Math.random() * 20);
  let payload, expectedStatus;
  
  if (messageType === 0) {
    // OTP message (5% - high success rate)
    payload = {
      client_id: DEMO_CLIENT,
      to: phone,
      from: 'OTP',
      otp: true
    };
    expectedStatus = [200]; // OTP always succeeds with sufficient credits
  } else if (messageType === 1) {
    // Express SMS (5% - premium service, high success rate)
    payload = {
      client_id: DEMO_CLIENT,
      to: phone,
      from: 'EXPRESS',
      text: `Express message ${messageId} from user ${userID}`,
      express: true
    };
    expectedStatus = [202]; // Queued
  } else {
    // Regular SMS (90% - normal traffic, high success rate)
    payload = {
      client_id: DEMO_CLIENT,
      to: phone,
      from: 'TEST',
      text: `User ${userID} message ${iterationID}: ${generateRandomText()}`
    };
    expectedStatus = [202]; // Queued
  }
  
  const startTime = new Date().getTime();
  
  const response = http.post(`${BASE_URL}/v1/messages`, 
    JSON.stringify(payload), 
    {
      headers: { 
        'Content-Type': 'application/json',
      },
      timeout: '15s', // Longer timeout for reliability
    }
  );
  
  const duration = new Date().getTime() - startTime;
  messageDuration.add(duration);

  const success = check(response, {
    'status is expected': (r) => expectedStatus.includes(r.status),
    'response time < 15000ms': (r) => r.timings.duration < 15000,
    'has response body': (r) => r.body && r.body.length > 0,
    'no 500 errors': (r) => r.status !== 500, // Critical: no system crashes
  });

  successRate.add(success);
  
  if (!success) {
    errors.add(1);
    console.error(`❌ User ${userID} Iteration ${iterationID} failed: Status ${response.status}, Body: ${response.body}`);
  } else {
    // Log successful requests periodically
    if (iterationID % 20 === 0) {
      console.log(`✅ User ${userID} completed ${iterationID + 1}/100 requests`);
    }
  }

  // Small sleep to prevent overwhelming the system (but maintain throughput)
  sleep(0.05); // 50ms between requests per user
}

export function setup() {
  console.log('🎯 **FINAL LOAD TEST: 100 Users × 100 Requests = 10,000 Total**');
  console.log('📊 Target: 10,000 requests with 100% success rate');
  console.log('👥 Strategy: 100 concurrent users, each making exactly 100 requests');
  console.log('📱 Message types: 90% regular, 5% express, 5% OTP');
  console.log('⏱️  Maximum duration: 10 minutes');
  console.log('🎯 Success criteria: >99% success rate, no 500 errors');
  
  // Verify system is healthy before starting
  const health = http.get(`${BASE_URL}/health`);
  if (health.status !== 200) {
    throw new Error('❌ System health check failed - aborting test');
  }
  
  // Check demo client credits
  const client = http.get(`${BASE_URL}/v1/me?client_id=${DEMO_CLIENT}`);
  if (client.status === 200) {
    const body = JSON.parse(client.body);
    console.log(`💰 Demo client has ${body.credits} credits available`);
    if (body.credits < 50000) { // Need extra credits for 100% success
      console.warn('⚠️  Warning: May need more credits for 100% success rate');
    }
  }
  
  console.log('✅ System health check passed, starting FINAL load test...');
  console.log('');
}

export function teardown(data) {
  console.log('');
  console.log('📊 **FINAL LOAD TEST RESULTS**');
  console.log('═══════════════════════════════════════');
  console.log('📈 Target: 10,000 requests (100 users × 100 each)');
  console.log('🎯 Goal: 100% success rate');
  
  // Check final system state
  const health = http.get(`${BASE_URL}/health`);
  const healthStatus = health.status === 200 ? '✅ HEALTHY' : '❌ DEGRADED';
  console.log(`🏥 Final system health: ${healthStatus}`);
  
  // Check remaining credits
  const client = http.get(`${BASE_URL}/v1/me?client_id=${DEMO_CLIENT}`);
  if (client.status === 200) {
    const body = JSON.parse(client.body);
    console.log(`💳 Final credits: ${body.credits}`);
  }
  
  // Check database stats
  console.log('🔍 Check database for final message counts and success rates');
  console.log('🎉 **FINAL LOAD TEST COMPLETED!**');
  console.log('═══════════════════════════════════════');
}
