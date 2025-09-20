import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { randomString, randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

// Custom metrics for SMS Gateway
const smsSuccessRate = new Rate('sms_success_rate');
const smsLatency = new Trend('sms_latency');
const otpSuccessRate = new Rate('otp_success_rate');
const expressSuccessRate = new Rate('express_success_rate');
const creditErrors = new Counter('credit_errors');
const queuedMessages = new Counter('queued_messages');
const immediateMessages = new Counter('immediate_messages');

// Test configuration
export let options = {
  scenarios: {
    // Scenario 1: Smoke test (basic functionality)
    smoke_test: {
      executor: 'constant-vus',
      vus: 1,
      duration: '30s',
      tags: { test_type: 'smoke' },
      env: { SCENARIO: 'smoke' },
    },
    
    // Scenario 2: Load test (normal traffic)
    load_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 10 },   // Ramp up to 10 users
        { duration: '5m', target: 10 },   // Stay at 10 users
        { duration: '2m', target: 20 },   // Ramp up to 20 users
        { duration: '5m', target: 20 },   // Stay at 20 users
        { duration: '2m', target: 0 },    // Ramp down
      ],
      tags: { test_type: 'load' },
      env: { SCENARIO: 'load' },
    },
    
    // Scenario 3: Stress test (high traffic)
    stress_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 50 },   // Ramp up to 50 users
        { duration: '5m', target: 50 },   // Stay at 50 users
        { duration: '2m', target: 100 },  // Ramp up to 100 users
        { duration: '5m', target: 100 },  // Stay at 100 users
        { duration: '2m', target: 0 },    // Ramp down
      ],
      tags: { test_type: 'stress' },
      env: { SCENARIO: 'stress' },
    },
    
    // Scenario 4: Spike test (sudden traffic bursts)
    spike_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 10 },   // Normal load
        { duration: '1m', target: 200 },  // Spike to 200 users
        { duration: '3m', target: 200 },  // Stay at spike
        { duration: '1m', target: 10 },   // Back to normal
        { duration: '2m', target: 0 },    // Ramp down
      ],
      tags: { test_type: 'spike' },
      env: { SCENARIO: 'spike' },
    },
    
    // Scenario 5: Volume test (100 clients × 1000 messages)
    volume_test: {
      executor: 'per-vu-iterations',
      vus: 100,
      iterations: 1000,
      maxDuration: '30m',
      tags: { test_type: 'volume' },
      env: { SCENARIO: 'volume' },
    },
  },
  
  thresholds: {
    // Overall performance thresholds
    http_req_duration: ['p(95)<2000', 'p(99)<5000'], // 95% under 2s, 99% under 5s
    http_req_failed: ['rate<0.05'],                   // Less than 5% failures
    
    // SMS-specific thresholds
    sms_success_rate: ['rate>0.95'],                  // 95% SMS success rate
    otp_success_rate: ['rate>0.98'],                  // 98% OTP success rate (higher requirement)
    express_success_rate: ['rate>0.97'],             // 97% Express SMS success rate
    
    // Latency thresholds by test type
    'http_req_duration{test_type:smoke}': ['p(95)<500'],
    'http_req_duration{test_type:load}': ['p(95)<1000'],
    'http_req_duration{test_type:stress}': ['p(95)<2000'],
    'http_req_duration{test_type:spike}': ['p(95)<3000'],
  },
};

// Test configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const CLIENT_ID = __ENV.CLIENT_ID || '550e8400-e29b-41d4-a716-446655440000';

// Message templates
const MESSAGE_TYPES = {
  REGULAR: 'regular',
  EXPRESS: 'express',
  OTP: 'otp',
};

export default function() {
  const scenario = __ENV.SCENARIO || 'load';
  
  switch(scenario) {
    case 'smoke':
      smokeTest();
      break;
    case 'load':
      loadTest();
      break;
    case 'stress':
      stressTest();
      break;
    case 'spike':
      spikeTest();
      break;
    case 'volume':
      volumeTest();
      break;
    default:
      mixedWorkload();
  }
}

// Smoke test: Basic functionality verification
function smokeTest() {
  console.log('🔍 Running smoke test...');
  
  // Test 1: Health check
  let healthResponse = http.get(`${BASE_URL}/health`);
  check(healthResponse, {
    'health check status is 200': (r) => r.status === 200,
    'health check response time < 100ms': (r) => r.timings.duration < 100,
  });
  
  // Test 2: API documentation
  let docsResponse = http.get(`${BASE_URL}/docs`);
  check(docsResponse, {
    'docs endpoint status is 200': (r) => r.status === 200,
  });
  
  // Test 3: Client info
  let clientResponse = http.get(`${BASE_URL}/v1/me?client_id=${CLIENT_ID}`);
  check(clientResponse, {
    'client info status is 200': (r) => r.status === 200,
    'client has credits': (r) => {
      const body = JSON.parse(r.body);
      return body.credits > 0;
    },
  });
  
  // Test 4: Send regular SMS
  sendRegularSMS();
  
  // Test 5: Send OTP SMS
  sendOTPSMS();
  
  // Test 6: Send Express SMS
  sendExpressSMS();
  
  sleep(1);
}

// Load test: Normal traffic simulation
function loadTest() {
  const messageType = getRandomMessageType([60, 30, 10]); // 60% regular, 30% express, 10% OTP
  
  switch(messageType) {
    case MESSAGE_TYPES.REGULAR:
      sendRegularSMS();
      break;
    case MESSAGE_TYPES.EXPRESS:
      sendExpressSMS();
      break;
    case MESSAGE_TYPES.OTP:
      sendOTPSMS();
      break;
  }
  
  sleep(randomIntBetween(1, 3)); // Random delay between requests
}

// Stress test: High load simulation
function stressTest() {
  const messageType = getRandomMessageType([50, 35, 15]); // More express and OTP under stress
  
  switch(messageType) {
    case MESSAGE_TYPES.REGULAR:
      sendRegularSMS();
      break;
    case MESSAGE_TYPES.EXPRESS:
      sendExpressSMS();
      break;
    case MESSAGE_TYPES.OTP:
      sendOTPSMS();
      break;
  }
  
  sleep(randomIntBetween(0.5, 1.5)); // Shorter delays under stress
}

// Spike test: Sudden load bursts
function spikeTest() {
  // During spike, send more critical messages (OTP and Express)
  const messageType = getRandomMessageType([30, 40, 30]); // More critical messages
  
  switch(messageType) {
    case MESSAGE_TYPES.REGULAR:
      sendRegularSMS();
      break;
    case MESSAGE_TYPES.EXPRESS:
      sendExpressSMS();
      break;
    case MESSAGE_TYPES.OTP:
      sendOTPSMS();
      break;
  }
  
  // No sleep during spike to maximize load
}

// Volume test: High volume simulation (100 clients × 1000 messages)
function volumeTest() {
  const vuId = __VU;
  const iteration = __ITER;
  
  console.log(`📊 VU ${vuId} sending message ${iteration + 1}/1000`);
  
  // Distribute message types evenly for volume test
  const messageType = getMessageTypeByIteration(iteration);
  
  switch(messageType) {
    case MESSAGE_TYPES.REGULAR:
      sendRegularSMS(vuId, iteration);
      break;
    case MESSAGE_TYPES.EXPRESS:
      sendExpressSMS(vuId, iteration);
      break;
    case MESSAGE_TYPES.OTP:
      sendOTPSMS(vuId, iteration);
      break;
  }
  
  // Small delay to avoid overwhelming the system
  sleep(0.1);
}

// Mixed workload: Realistic traffic patterns
function mixedWorkload() {
  const messageType = getRandomMessageType([55, 30, 15]); // Realistic distribution
  
  switch(messageType) {
    case MESSAGE_TYPES.REGULAR:
      sendRegularSMS();
      break;
    case MESSAGE_TYPES.EXPRESS:
      sendExpressSMS();
      break;
    case MESSAGE_TYPES.OTP:
      sendOTPSMS();
      break;
  }
  
  sleep(randomIntBetween(1, 2));
}

// SMS sending functions
function sendRegularSMS(vuId = __VU, iteration = __ITER) {
  const phoneNumber = generatePhoneNumber(vuId, iteration);
  const messageText = `Regular SMS #${iteration} from VU ${vuId} - ${randomString(20)}`;
  
  const payload = {
    client_id: CLIENT_ID,
    to: phoneNumber,
    from: 'K6_TEST',
    text: messageText,
    express: false,
  };
  
  const response = http.post(`${BASE_URL}/v1/messages`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    tags: { message_type: 'regular' },
  });
  
  const success = check(response, {
    'regular SMS status is 202': (r) => r.status === 202,
    'regular SMS response time < 1000ms': (r) => r.timings.duration < 1000,
    'regular SMS has message_id': (r) => {
      const body = JSON.parse(r.body);
      return body.message_id !== undefined;
    },
  });
  
  smsSuccessRate.add(success);
  smsLatency.add(response.timings.duration);
  
  if (response.status === 202) {
    queuedMessages.add(1);
  }
  
  if (response.status === 402) {
    creditErrors.add(1);
  }
}

function sendExpressSMS(vuId = __VU, iteration = __ITER) {
  const phoneNumber = generatePhoneNumber(vuId, iteration);
  const messageText = `🚨 URGENT EXPRESS SMS #${iteration} from VU ${vuId}`;
  
  const payload = {
    client_id: CLIENT_ID,
    to: phoneNumber,
    from: 'EXPRESS',
    text: messageText,
    express: true,
  };
  
  const response = http.post(`${BASE_URL}/v1/messages`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    tags: { message_type: 'express' },
  });
  
  const success = check(response, {
    'express SMS status is 202': (r) => r.status === 202,
    'express SMS response time < 800ms': (r) => r.timings.duration < 800,
    'express SMS has message_id': (r) => {
      const body = JSON.parse(r.body);
      return body.message_id !== undefined;
    },
  });
  
  expressSuccessRate.add(success);
  smsLatency.add(response.timings.duration);
  
  if (response.status === 202) {
    queuedMessages.add(1);
  }
  
  if (response.status === 402) {
    creditErrors.add(1);
  }
}

function sendOTPSMS(vuId = __VU, iteration = __ITER) {
  const phoneNumber = generatePhoneNumber(vuId, iteration);
  
  const payload = {
    client_id: CLIENT_ID,
    to: phoneNumber,
    from: 'OTP_BANK',
    otp: true,
  };
  
  const response = http.post(`${BASE_URL}/v1/messages`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    tags: { message_type: 'otp' },
  });
  
  const success = check(response, {
    'OTP SMS status is 200 or 503': (r) => r.status === 200 || r.status === 503,
    'OTP SMS response time < 500ms': (r) => r.timings.duration < 500,
  });
  
  // OTP-specific checks
  if (response.status === 200) {
    const otpSuccess = check(response, {
      'OTP delivered immediately': (r) => r.status === 200,
      'OTP has code': (r) => {
        const body = JSON.parse(r.body);
        return body.otp_code !== undefined;
      },
    });
    
    otpSuccessRate.add(otpSuccess);
    immediateMessages.add(1);
  } else if (response.status === 503) {
    otpSuccessRate.add(false);
  }
  
  smsLatency.add(response.timings.duration);
  
  if (response.status === 402) {
    creditErrors.add(1);
  }
}

// Utility functions
function getRandomMessageType(weights) {
  const [regular, express, otp] = weights;
  const total = regular + express + otp;
  const random = Math.random() * total;
  
  if (random < regular) return MESSAGE_TYPES.REGULAR;
  if (random < regular + express) return MESSAGE_TYPES.EXPRESS;
  return MESSAGE_TYPES.OTP;
}

function getMessageTypeByIteration(iteration) {
  // Cycle through message types for even distribution
  const cycle = iteration % 10;
  if (cycle < 6) return MESSAGE_TYPES.REGULAR;   // 60%
  if (cycle < 9) return MESSAGE_TYPES.EXPRESS;   // 30%
  return MESSAGE_TYPES.OTP;                      // 10%
}

function generatePhoneNumber(vuId, iteration) {
  // Generate unique phone numbers to avoid conflicts
  const base = 1000000000; // Start with +1 000 000 000
  const unique = (vuId * 10000) + (iteration % 10000);
  return `+1${base + unique}`;
}

// Custom setup and teardown
export function setup() {
  console.log('🚀 Starting SMS Gateway K6 Load Test');
  console.log(`📊 Base URL: ${BASE_URL}`);
  console.log(`🆔 Client ID: ${CLIENT_ID}`);
  
  // Verify system is ready
  let healthResponse = http.get(`${BASE_URL}/health`);
  if (healthResponse.status !== 200) {
    throw new Error(`System not ready: ${healthResponse.status}`);
  }
  
  // Check client credits
  let clientResponse = http.get(`${BASE_URL}/v1/me?client_id=${CLIENT_ID}`);
  if (clientResponse.status === 200) {
    const body = JSON.parse(clientResponse.body);
    console.log(`💰 Client credits: ${body.credits}`);
    
    if (body.credits < 10000) {
      console.warn('⚠️  Low client credits, test may fail');
    }
  }
  
  return { startTime: new Date().toISOString() };
}

export function teardown(data) {
  console.log('🏁 SMS Gateway K6 Load Test Completed');
  console.log(`📅 Started: ${data.startTime}`);
  console.log(`📅 Ended: ${new Date().toISOString()}`);
  
  // Final system check
  let healthResponse = http.get(`${BASE_URL}/health`);
  console.log(`🔍 Final health check: ${healthResponse.status}`);
  
  // Check remaining credits
  let clientResponse = http.get(`${BASE_URL}/v1/me?client_id=${CLIENT_ID}`);
  if (clientResponse.status === 200) {
    const body = JSON.parse(clientResponse.body);
    console.log(`💰 Remaining credits: ${body.credits}`);
  }
}
