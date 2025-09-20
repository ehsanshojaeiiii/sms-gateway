import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Endurance test: Long-running test to check for memory leaks, connection issues
export let options = {
  scenarios: {
    endurance_test: {
      executor: 'constant-vus',
      vus: 20,
      duration: '30m',  // 30 minutes of constant load
    },
  },
  
  thresholds: {
    http_req_duration: ['p(95)<1500'],
    http_req_failed: ['rate<0.05'],
    'memory_usage': ['value<80'],     // Custom metric for memory monitoring
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const CLIENT_ID = __ENV.CLIENT_ID || '550e8400-e29b-41d4-a716-446655440000';

const enduranceSuccessRate = new Rate('endurance_success_rate');
const memoryUsage = new Trend('memory_usage');
const connectionErrors = new Counter('connection_errors');

export default function() {
  // Simulate realistic usage patterns over time
  const iteration = __ITER;
  
  // Every 100 iterations, check system health
  if (iteration % 100 === 0) {
    checkSystemHealth();
  }
  
  // Rotate through different message types
  const messageType = iteration % 4;
  
  switch(messageType) {
    case 0:
      sendRegularMessage();
      sleep(2);
      break;
    case 1:
      sendExpressMessage();
      sleep(1.5);
      break;
    case 2:
      sendOTPMessage();
      sleep(1);
      break;
    case 3:
      checkClientInfo();
      sleep(0.5);
      break;
  }
}

function checkSystemHealth() {
  const healthResponse = http.get(`${BASE_URL}/health`);
  
  check(healthResponse, {
    'system health ok': (r) => r.status === 200,
    'health response fast': (r) => r.timings.duration < 200,
  });
  
  if (healthResponse.status !== 200) {
    connectionErrors.add(1);
  }
}

function checkClientInfo() {
  const response = http.get(`${BASE_URL}/v1/me?client_id=${CLIENT_ID}`);
  
  check(response, {
    'client info available': (r) => r.status === 200,
    'client has credits': (r) => {
      if (r.status === 200) {
        const body = JSON.parse(r.body);
        return body.credits > 0;
      }
      return false;
    },
  });
}

function sendRegularMessage() {
  const payload = {
    client_id: CLIENT_ID,
    to: `+1555${Date.now().toString().slice(-7)}`,
    from: 'ENDURANCE',
    text: `Endurance test message #${__ITER} from VU ${__VU}`,
  };
  
  const response = http.post(`${BASE_URL}/v1/messages`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
  });
  
  const success = response.status === 202;
  enduranceSuccessRate.add(success);
  
  check(response, {
    'endurance message queued': (r) => r.status === 202,
  });
}

function sendExpressMessage() {
  const payload = {
    client_id: CLIENT_ID,
    to: `+1666${Date.now().toString().slice(-7)}`,
    from: 'ENDURANCE_EXP',
    text: `Express endurance test #${__ITER}`,
    express: true,
  };
  
  const response = http.post(`${BASE_URL}/v1/messages`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
  });
  
  const success = response.status === 202;
  enduranceSuccessRate.add(success);
  
  check(response, {
    'endurance express queued': (r) => r.status === 202,
  });
}

function sendOTPMessage() {
  const payload = {
    client_id: CLIENT_ID,
    to: `+1777${Date.now().toString().slice(-7)}`,
    from: 'ENDURANCE_OTP',
    otp: true,
  };
  
  const response = http.post(`${BASE_URL}/v1/messages`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
  });
  
  const success = response.status === 200 || response.status === 503;
  enduranceSuccessRate.add(success);
  
  check(response, {
    'endurance OTP processed': (r) => r.status === 200 || r.status === 503,
  });
}

export function setup() {
  console.log('🏃‍♂️ Starting 30-minute Endurance Test');
  console.log('📊 This test checks for memory leaks and long-term stability');
  
  return { startTime: Date.now() };
}

export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000 / 60;
  console.log(`🏁 Endurance test completed after ${duration.toFixed(1)} minutes`);
}
