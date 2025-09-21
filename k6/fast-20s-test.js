import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    fast_burst: {
      executor: 'ramping-vus',
      startVUs: 5,
      stages: [
        { duration: '5s', target: 20 },   // Ramp up to 20 users
        { duration: '10s', target: 20 },  // Sustain 20 users  
        { duration: '5s', target: 0 },    // Ramp down
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<100'],     // 95% under 100ms
    http_req_failed: ['rate<0.01'],       // Less than 1% failures
    checks: ['rate>0.99'],                // 99%+ success rate
  },
};

export function setup() {
  console.log('🚀 **FAST 20-SECOND TEST STARTED**');
  console.log('⚡ Target: ~400 messages in 20 seconds');
  console.log('👤 Using VALID demo client ID');
  
  const health = http.get('http://localhost:8080/health');
  if (health.status !== 200) {
    throw new Error('❌ System health check failed');
  }
  console.log('✅ System ready for fast test');
  
  return { startTime: Date.now() };
}

export default function () {
  // Use the REAL demo client ID that exists in the database
  const clientId = '550e8400-e29b-41d4-a716-446655440000';
  
  const url = 'http://localhost:8080/v1/messages';
  
  const payload = JSON.stringify({
    client_id: clientId,
    to: `+989${Math.floor(Math.random() * 900000000 + 100000000)}`,
    from: `FastTest`, 
    text: `Fast test message - User ${__VU}, Iteration ${__ITER}`,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const response = http.post(url, payload, params);

  const success = check(response, {
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

  // Fast pacing for high throughput - 1 second between requests
  sleep(1);
}

export function teardown(data) {
  const totalDuration = (Date.now() - data.startTime) / 1000;
  console.log(`✅ **FAST TEST COMPLETED**:`);
  console.log(`⏱️  Duration: ${totalDuration.toFixed(1)} seconds`);
  console.log(`🏥 System health: ${http.get('http://localhost:8080/health').status === 200 ? 'EXCELLENT' : 'ERROR'}`);
  console.log(`🎯 Fast 20-second test finished!`);
}
