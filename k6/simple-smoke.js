import http from 'k6/http';
import { check } from 'k6';

export let options = {
  vus: 1,
  duration: '30s',
};

const CLIENT_ID = '550e8400-e29b-41d4-a716-446655440000';
const BASE_URL = 'http://localhost:8080';

export default function () {
  // Test API health
  let healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, {
    'health check status is 200': (r) => r.status === 200,
  });

  // Test SMS sending
  let smsPayload = JSON.stringify({
    client_id: CLIENT_ID,
    to: '+1234567890',
    from: 'K6TEST',
    text: `Test message at ${new Date().toISOString()}`
  });

  let smsRes = http.post(`${BASE_URL}/v1/messages`, smsPayload, {
    headers: { 'Content-Type': 'application/json' },
  });

  check(smsRes, {
    'SMS send status is 202': (r) => r.status === 202,
    'SMS has message_id': (r) => r.json('message_id') !== undefined,
  });
}
