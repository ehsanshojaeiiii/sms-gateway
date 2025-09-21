import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    load_test: {
      executor: 'per-vu-iterations',
      vus: 100,            // 100 virtual users
      iterations: 100,     // Each user sends 100 requests = 10,000 total
      maxDuration: '10m',  // Maximum 10 minutes
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<5000'], // 95% of requests under 5s
    http_req_failed: ['rate<0.1'],     // Less than 10% failures
  },
};

export default function () {
  const url = 'http://localhost:8080/v1/messages';
  
  const payload = JSON.stringify({
    client_id: '550e8400-e29b-41d4-a716-446655440000',
    to: `+989${Math.floor(Math.random() * 900000000 + 100000000)}`, // Random Iranian mobile
    from: 'TestSender',
    text: `Load test message ${__VU}-${__ITER} at ${new Date().toISOString()}`,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const response = http.post(url, payload, params);
  
  check(response, {
    'status is 202': (r) => r.status === 202,
    'has message_id': (r) => JSON.parse(r.body).message_id !== undefined,
    'status is QUEUED': (r) => JSON.parse(r.body).status === 'QUEUED',
  });

  // Small random delay to simulate realistic load
  sleep(Math.random() * 0.1);
}
