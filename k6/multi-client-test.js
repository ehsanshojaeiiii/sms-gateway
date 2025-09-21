import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    multi_client_load: {
      executor: 'per-vu-iterations',
      vus: 10,           // 10 virtual users (one per client)
      iterations: 100,   // Each user sends 100 messages = 1,000 total
      maxDuration: '5m',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000'],
    http_req_failed: ['rate<0.1'],
  },
};

// 10 test clients with guaranteed credits
const CLIENT_IDS = [
  '11111111-1111-1111-1111-111111111111',
  '22222222-2222-2222-2222-222222222222', 
  '33333333-3333-3333-3333-333333333333',
  '44444444-4444-4444-4444-444444444444',
  '55555555-5555-5555-5555-555555555555',
  '66666666-6666-6666-6666-666666666666',
  '77777777-7777-7777-7777-777777777777',
  '88888888-8888-8888-8888-888888888888',
  '99999999-9999-9999-9999-999999999999',
  'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
];

export default function () {
  // Each VU uses a specific client ID (VU 1 uses client 0, VU 2 uses client 1, etc.)
  const clientId = CLIENT_IDS[(__VU - 1) % CLIENT_IDS.length];
  
  const url = 'http://localhost:8080/v1/messages';
  
  const payload = JSON.stringify({
    client_id: clientId,
    to: `+989${Math.floor(Math.random() * 900000000 + 100000000)}`, 
    from: `Client${__VU}`,
    text: `Multi-client test message from VU ${__VU}, iteration ${__ITER}`,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
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

  // Small delay to simulate realistic usage
  sleep(Math.random() * 0.1);
}
