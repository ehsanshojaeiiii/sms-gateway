import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Burst test: Simulate sudden traffic spikes (Black Friday, breaking news, etc.)
export let options = {
  scenarios: {
    burst_traffic: {
      executor: 'ramping-arrival-rate',
      startRate: 10,     // Start with 10 requests/second
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 200,
      stages: [
        { duration: '30s', target: 10 },    // Normal traffic
        { duration: '10s', target: 500 },   // Sudden burst!
        { duration: '60s', target: 500 },   // Sustained burst
        { duration: '30s', target: 50 },    // Gradual decrease
        { duration: '30s', target: 10 },    // Back to normal
      ],
    },
  },
  
  thresholds: {
    http_req_duration: ['p(95)<3000'],    // Allow higher latency during burst
    http_req_failed: ['rate<0.10'],       // Allow 10% failure during extreme burst
    'http_req_duration{scenario:burst_traffic}': ['p(99)<5000'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const CLIENT_ID = __ENV.CLIENT_ID || '550e8400-e29b-41d4-a716-446655440000';

const burstSuccessRate = new Rate('burst_success_rate');
const burstLatency = new Trend('burst_latency');

export default function() {
  // During burst, prioritize critical messages
  const messageType = Math.random();
  
  if (messageType < 0.4) {
    sendOTPMessage();        // 40% OTP (critical)
  } else if (messageType < 0.7) {
    sendExpressMessage();    // 30% Express (urgent)
  } else {
    sendRegularMessage();    // 30% Regular
  }
}

function sendOTPMessage() {
  const payload = {
    client_id: CLIENT_ID,
    to: `+1${Date.now()}${Math.floor(Math.random() * 1000)}`,
    from: 'BURST_OTP',
    otp: true,
  };
  
  const response = http.post(`${BASE_URL}/v1/messages`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    tags: { burst_type: 'otp' },
  });
  
  const success = response.status === 200 || response.status === 503;
  burstSuccessRate.add(success);
  burstLatency.add(response.timings.duration);
  
  check(response, {
    'OTP burst response ok': (r) => r.status === 200 || r.status === 503,
  });
}

function sendExpressMessage() {
  const payload = {
    client_id: CLIENT_ID,
    to: `+1${Date.now()}${Math.floor(Math.random() * 1000)}`,
    from: 'BURST_EXPRESS',
    text: '🚨 BREAKING: Urgent notification during traffic burst',
    express: true,
  };
  
  const response = http.post(`${BASE_URL}/v1/messages`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    tags: { burst_type: 'express' },
  });
  
  const success = response.status === 202;
  burstSuccessRate.add(success);
  burstLatency.add(response.timings.duration);
  
  check(response, {
    'Express burst queued': (r) => r.status === 202,
  });
}

function sendRegularMessage() {
  const payload = {
    client_id: CLIENT_ID,
    to: `+1${Date.now()}${Math.floor(Math.random() * 1000)}`,
    from: 'BURST_REGULAR',
    text: 'Regular message during burst traffic',
  };
  
  const response = http.post(`${BASE_URL}/v1/messages`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    tags: { burst_type: 'regular' },
  });
  
  const success = response.status === 202;
  burstSuccessRate.add(success);
  burstLatency.add(response.timings.duration);
  
  check(response, {
    'Regular burst queued': (r) => r.status === 202,
  });
}
