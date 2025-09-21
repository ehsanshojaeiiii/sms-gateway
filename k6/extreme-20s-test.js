import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    extreme_burst: {
      executor: 'ramping-vus',
      startVUs: 10,
      stages: [
        { duration: '2s', target: 50 },    // Quick ramp to 50 users
        { duration: '16s', target: 50 },   // Sustain 50 users for 16s  
        { duration: '2s', target: 0 },     // Quick ramp down
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],     // 95% under 500ms
    http_req_failed: ['rate<0.05'],       // Less than 5% failures (accounting for rate limits)
    checks: ['rate>0.95'],                // 95%+ success rate
  },
};

export function setup() {
  console.log('⚡ **EXTREME 20-SECOND TEST: 2,300 SMS (69k scaled down)**');
  console.log('🎯 Target: 115 SMS/second sustained rate');
  console.log('👥 50 concurrent users × ~46 messages each');
  
  // Health check
  const health = http.get('http://localhost:8080/health');
  if (health.status !== 200) {
    throw new Error('System not ready');
  }
  
  console.log('✅ System ready - starting extreme test...');
  return { startTime: Date.now() };
}

export default function () {
  // Demo client ID (has sufficient credits)
  const clientId = '550e8400-e29b-41d4-a716-446655440000';
  
  const payload = {
    client_id: clientId,
    to: `+98912345${Math.floor(Math.random() * 10000).toString().padStart(4, '0')}`,
    from: "EXTREME",
    text: `Extreme test ${__ITER} from VU ${__VU} - scaled 69k test`
  };

  const response = http.post('http://localhost:8080/v1/messages', JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    timeout: '10s',
  });

  const success = check(response, {
    'status is 202': (r) => r.status === 202,
    'not rate limited': (r) => r.status !== 429,
    'has message_id': (r) => r.json().message_id !== undefined,
    'status is QUEUED': (r) => r.json().status === 'QUEUED',
  });

  // Small sleep to prevent overwhelming (but still aggressive)
  sleep(0.1);
}

export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000;
  console.log(`⏱️  Test completed in ${duration.toFixed(1)} seconds`);
  console.log('📊 Expected: ~2,300 messages');
  console.log('🔍 Check database for actual results...');
}
