import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom metrics
export const errors = new Counter('errors');
export const successRate = new Rate('success_rate');
export const messageDuration = new Trend('message_duration');

// 100 test client IDs - each will send 690 requests (69,000 total)
const CLIENT_IDS = [
    '4585e6e4-76d3-43a8-9e44-9964c827f694', '0c0eb22b-70d8-48ba-8ddf-5dc7e0edcc78',
    '6023a5a4-76b9-4dc0-9caa-078cc5618f3a', '3087d2e4-7d73-44dc-b3e8-9f62952bd3a0',
    '020fbb19-0582-4e6f-87ab-60f799ab22a9', '0302ec1b-cb5c-4869-854e-fa2f041474f6',
    '82acf128-b90f-4d52-82af-608944dfda7e', 'c63de7a7-88fc-4c05-94ca-43cb4da91729',
    '6a67643c-786c-42cb-84d8-1b4e2fd90f1a', '97fc8b0c-7d07-47c7-8307-4fadc7c214d9',
    '667c2d25-f6bf-4aa1-ad2f-8f5b70f2332d', '014a3824-3ad7-4f7f-a6ea-072087b4b150',
    'fc18def7-293d-4236-b1ec-25b40a023369', '651a798f-547b-4e03-965b-bd46f4609b5c',
    '1ea5f9d3-ee18-4fc4-b361-c243b5fc04b9', '25af29d9-fa75-447e-b772-78cced3a4935',
    'e0d1293f-8ff0-4bf6-83f5-418f716a70cb', '5c09d753-fbe4-4a03-a402-188646b5d5c3',
    'ad6bbee6-5eb7-45ba-a3a7-c30fca234c02', '5f0dd95f-7345-499f-8f94-f15dfd52a725',
    'fcf61149-6e4f-4aef-9779-4d6d9d340b2e', '480a0417-8360-4d39-8da9-1030a2d0b16a',
    '28b4c3c0-019a-4b57-a8d3-66d07288d138', 'd1bfd6b8-84c8-4146-830c-d62cdde3ff32',
    '3440064e-6705-4aa6-b442-13af19f6a99a', '10213245-d8a9-43aa-965b-6b830b9f4240',
    'eff5fd92-0c03-40fd-ad1f-211553165aa4', 'd005b456-8956-40fc-9d1e-d96557de26e1',
    '068ac3f7-6e6a-4e96-92ee-34818335c33b', '2d34b829-8cb5-4739-bcee-97fd949a0a84',
    '86498f80-66fa-4931-b35e-dc3feb1b69d3', '7605c4f1-4c0a-48b9-9258-fc81dc2b9b81',
    'daceb12d-bd02-4819-b8cf-886be6b6f0c5', '49f85906-f477-48b2-acdf-3aabd2ac2464',
    'd663c371-4ec9-48e8-9de8-9a13e03523dc', '74e6d138-2e60-4aba-b8dd-e18a0f4cf9e6',
    'c3806607-e396-41c4-ba67-eda4dd88a5eb', '6b0d3e04-4696-407f-ade7-a28ae4067517',
    '59f198f1-abc7-492c-9fdb-e566593f41f1', '3a708e1e-fa61-4f74-829c-59b1bbe4cdad',
    'f8281400-db8b-4bb3-b23e-028dc56ee2f4', '05ece5e9-325a-45c4-879a-0b66599711e0',
    'ee7944ed-3ca6-4f29-a20e-13b99c6f3839', '377ffb84-9136-4c59-9b84-a9e1527140b9',
    '24ab05ad-8223-49d6-bfe6-a159ceb90509', '395a95f2-fdaa-4fbd-bb8a-5dcdbda3b573',
    'b7d86663-8cd3-4dea-936d-09efb101e948', 'bb913e99-5a3f-43d2-8030-7a7dcde90598',
    '409cb1f0-557f-4a58-b980-718499dbe7d6', '2fb566a6-13cc-44be-886e-96580e2f4d70',
    '5f0776e8-abd9-469e-80f1-0196466884b9', 'b7d9fa00-154b-4975-9525-5ece17917511',
    '57bc03cb-b1a0-4d6f-8a5c-8ec743a5f022', '3a094fed-ff35-47c2-874e-b009f13b0b8a',
    'af9fa494-aec7-4ea9-85be-3002117d92c0', '0190cf55-0c72-4d02-a20d-f152dd23f2a9',
    '06b5ee78-0e52-4bd6-aabe-f4988966d661', 'fdcd8087-6d0d-407b-b6ef-7b57ed2f3e07',
    'cfabd7d8-9fa7-4673-a786-d40a3c26005a', '0498f575-4644-4511-991a-332e6204264c',
    '540c28b1-5b55-4eca-a3eb-c28db136bcd4', 'b54ac508-96d5-4da3-8e0b-e2f453f57f17',
    '81d167fd-2511-4bba-9b77-e507834a20ec', 'edf1c75d-a1f9-4046-b66a-3ca323e8b415',
    '2bd31c22-ce3b-4b1d-bc2c-9a8160fe1d58', '056f699e-eea1-46c5-81f0-057e123703dd',
    '2f86a98c-35fb-42d9-86cc-dc1aa9e7e6b9', 'ab30b232-5156-4646-9fa6-0d744938dfe0',
    'ef343885-996e-435f-ad5a-855b7f954ca0', '8c0f7db3-1dd3-4d0c-8eb5-fef15ff76d99',
    'eb2036f0-7a8d-44de-bc8b-43220270b10d', '41206945-04ba-4040-855c-be85e32f32bd',
    '0c51aeed-0d88-4e1e-a3ed-564d82e6387e', 'ab257bd6-64f0-4eae-b33b-3bcca3326fbe',
    'ce17044e-82f1-4bbe-b7cc-4f1063e6f3e5', '3cf34e99-a870-48cf-be15-a20a8dfe1a35',
    '29847fa9-b3c3-4664-9250-f40dc66e545e', '08f08d1a-c559-4c63-ad8e-834bb466159e',
    '2c0ee55e-5f46-450d-98f0-db78ae30a53d', '67dba8a2-ffc2-424a-82d2-acbe0e995447',
    'ca7cc634-7954-415d-8321-0b0e0f1345d3', '7f259532-658b-42f5-868c-507d70e4a8e8',
    'bb50bc6e-11be-4fa2-b35c-caba704ec367', '753bc75a-a478-4e3f-a2e9-fd5c43fd9677',
    '1b533130-da5e-40d0-a987-f43b1308abf5', '50c9a30e-618b-438c-9738-13ee2e37e2b7',
    'cb7402d4-d8a3-45a1-96c8-6e86763d5fa3', 'e7e5b5ae-8ffc-4d78-a3a3-c278d5f6a502',
    '1f1e1310-a019-4ebb-90a4-3a82d53d5ad3', 'ae1823c2-aba0-4a99-b816-bd9bd9d77471',
    '67df1649-c35c-45fa-b130-b5c176352c78', '56625976-2fbd-4d01-a431-c2fe65acd285',
    'dd639689-8db7-4490-9b29-9c68a56e1b78', '6f1e9f91-91a0-44aa-9a6b-744d64e7ee90',
    '62485dff-3a2f-419d-8787-ce2b208761d9', '43df88e5-fe84-4dbb-b51f-848bb50e3da7',
    '73693d6f-1e59-4a2f-a7c5-06d0074c0fe2', '977e4ea5-aab9-4a09-8b6d-ef89f33c0b59',
    'f0806e30-ed37-457e-afd2-3b71511a4a0d', '58e999a7-13a3-4e17-8263-50e375eddb8b'
];

// Configuration: 69,000 requests across 100 users in 10 minutes
// Each user sends 690 requests, target RPS: 115 requests/second total
export const options = {
  stages: [
    { duration: '1m', target: 20 },    // Ramp up slowly
    { duration: '1m', target: 50 },    // Increase load
    { duration: '1m', target: 80 },    // Near target load
    { duration: '1m', target: 100 },   // Target: 100 VUs (1 per client)
    { duration: '5m', target: 100 },   // Maintain load for main test
    { duration: '1m', target: 50 },    // Ramp down
    { duration: '30s', target: 0 },    // Complete
  ],
  thresholds: {
    'errors': ['count<6900'],           // Allow up to 10% errors
    'http_req_duration': ['p(95)<3000'], // 95% under 3 seconds
    'http_req_failed': ['rate<0.10'],   // Less than 10% failure rate
    'success_rate': ['rate>0.90'],      // At least 90% success rate
  },
};

// Pre-test system validation
export function setup() {
  console.log('🚀 Starting Multi-User Load Test Setup');
  console.log(`📊 Target: 69,000 requests across ${CLIENT_IDS.length} users`);
  console.log('🎯 Each user will send ~690 requests over 10 minutes');
  
  const healthCheck = http.get('http://localhost:8080/health');
  if (healthCheck.status !== 200) {
    throw new Error(`❌ System not healthy: ${healthCheck.status}`);
  }
  console.log('✅ System health check passed');
  
  // Validate that we have 100 clients
  console.log(`✅ Loaded ${CLIENT_IDS.length} client IDs for testing`);
  return { startTime: Date.now() };
}

// Main test function
export default function(data) {
  const startTime = new Date().getTime();
  
  // Each VU represents one client - use VU ID to select client
  const vuIndex = (__VU - 1) % CLIENT_IDS.length;
  const clientId = CLIENT_IDS[vuIndex];
  
  // Message types distribution: 80% regular, 15% express, 5% OTP
  const rand = Math.random();
  let messageType, endpoint, payload;
  
  if (rand < 0.80) {
    // Regular SMS (80%)
    endpoint = 'http://localhost:8080/v1/sms';
    payload = {
      to: `+989${String(Math.floor(Math.random() * 1000000000)).padStart(9, '0')}`,
      message: `Multi-user test message from client ${vuIndex + 1} at ${new Date().toISOString()}`,
      client_id: clientId
    };
    messageType = 'regular';
  } else if (rand < 0.95) {
    // Express SMS (15%)
    endpoint = 'http://localhost:8080/v1/sms';
    payload = {
      to: `+989${String(Math.floor(Math.random() * 1000000000)).padStart(9, '0')}`,
      message: `EXPRESS: Multi-user test from client ${vuIndex + 1}`,
      client_id: clientId,
      express: true
    };
    messageType = 'express';
  } else {
    // OTP SMS (5%)
    endpoint = 'http://localhost:8080/v1/otp';
    payload = {
      to: `+989${String(Math.floor(Math.random() * 1000000000)).padStart(9, '0')}`,
      template: `Your OTP code is: {{code}}`,
      client_id: clientId
    };
    messageType = 'otp';
  }

  const response = http.post(endpoint, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    timeout: '30s'
  });

  const duration = new Date().getTime() - startTime;
  messageDuration.add(duration);

  // Comprehensive response validation
  const success = check(response, {
    'status is 200 or 201': (r) => [200, 201].includes(r.status),
    'has message_id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.message_id !== undefined;
      } catch {
        return false;
      }
    },
    'has status': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status !== undefined;
      } catch {
        return false;
      }
    },
    'response time OK': (r) => r.timings.duration < 5000,
  });

  if (success) {
    successRate.add(1);
  } else {
    successRate.add(0);
    errors.add(1);
    
    // Log errors for debugging
    if (response.status === 402) {
      console.warn(`💳 Client ${vuIndex + 1} (${clientId}) out of credits`);
    } else if (response.status >= 500) {
      console.warn(`🚨 Server error ${response.status} for client ${vuIndex + 1}`);
    }
  }

  // Realistic pacing - vary between message types
  const sleepTime = messageType === 'express' ? 0.1 : 
                   messageType === 'otp' ? 0.5 : 0.3;
  sleep(sleepTime);
}

// Post-test validation and reporting
export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000;
  
  console.log('📊 Multi-User Load Test Summary:');
  console.log(`⏱️  Total Duration: ${duration.toFixed(2)} seconds`);
  console.log(`👥 Test Users: ${CLIENT_IDS.length} clients`);
  console.log(`📈 Target Requests: 69,000 across all users`);
  console.log(`🎯 Target RPS: 115 requests/second`);
  
  // Final system health check
  const healthCheck = http.get('http://localhost:8080/health');
  const healthStatus = healthCheck.status === 200 ? 'OK' : 'DEGRADED';
  console.log(`🏥 Final system health: ${healthStatus}`);
  
  // Sample credit check for first few clients
  for (let i = 0; i < Math.min(5, CLIENT_IDS.length); i++) {
    const creditCheck = http.get(`http://localhost:8080/v1/me?client_id=${CLIENT_IDS[i]}`);
    if (creditCheck.status === 200) {
      try {
        const credits = JSON.parse(creditCheck.body).credit_cents;
        console.log(`💳 Client ${i + 1} remaining credits: ${credits}`);
      } catch (e) {
        console.log(`💳 Client ${i + 1} credit check failed`);
      }
    }
  }
  
  console.log('🎉 Multi-user load test completed!');
}

// Export configuration for external monitoring
export { CLIENT_IDS };
