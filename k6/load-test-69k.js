import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    load_test: {
      executor: 'ramping-vus',
      startVUs: 10,
      stages: [
        { duration: '2m', target: 100 },   // Ramp up to 100 users
        { duration: '6m', target: 100 },   // Stay at 100 users for 6 minutes
        { duration: '2m', target: 0 },     // Ramp down
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000'],
    http_req_failed: ['rate<0.1'],
  },
};

export function setup() {
  console.log('🚀 Load Test: 69,000 SMS messages over 10 minutes');
  
  const health = http.get('http://localhost:8080/health');
  if (health.status !== 200) {
    throw new Error('System not ready');
  }
  
  return { startTime: Date.now() };
}

export default function () {
  const clientId = '550e8400-e29b-41d4-a716-446655440000';
  
  const url = 'http://localhost:8080/v1/messages';
  const payload = JSON.stringify({
    client_id: clientId,
    to: `+989${Math.floor(Math.random() * 900000000 + 100000000)}`,
    from: `LoadTest${__VU}`,
    text: `Load test message - User ${__VU}, Iteration ${__ITER}`,
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const response = http.post(url, payload, params);

  check(response, {
    'status is 202': (r) => r.status === 202,
    'has message_id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.message_id !== undefined;
      } catch {
        return false;
      }
    },
    'status is QUEUED': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status === 'QUEUED';
      } catch {
        return false;
      }
    },
  });

  // Target: ~690 messages per user over 10 minutes = ~1.15 messages per second per user
  sleep(0.8 + Math.random() * 0.4); // 0.8-1.2 second delay
}

export function teardown(data) {
  const totalDuration = (Date.now() - data.startTime) / 1000;
  console.log(`✅ Load test completed in ${totalDuration.toFixed(1)} seconds`);
  console.log(`🏥 System health: ${http.get('http://localhost:8080/health').status === 200 ? 'PASS' : 'FAIL'}`);
}
