import ws from 'k6/ws';
import { check } from 'k6';
import { Trend, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const roundtripTrend = new Trend('ws_roundtrip_latency_ms');
const errorCounter = new Counter('ws_errors');
const sentCounter = new Counter('ws_messages_sent');
const recvCounter = new Counter('ws_messages_received');

const tokenPool = new SharedArray('token_pool', function () {
  return JSON.parse(open('./tokens.json'));
});

const MSG_INTERVAL_MS = 1000
const URLS = __ENV.URLS;

if (!URLS) {
  throw new Error('URLS must be set.');
}

export const options = {
  stages: [
    { duration: '15s', target: 50 },
    { duration: '15s', target: 500 },
    { duration: '15s', target: 1000 },
  ],
  thresholds: {
    ws_roundtrip_latency_ms: ['p(95)<150', 'p(99)<300'],
    ws_errors: ['count<100'],
  },
};

export default function () {
  const userObj = tokenPool[__VU % tokenPool.length];
  const token = userObj.token;
  const conversationID = userObj.conversationID;

  const urls = URLS.split(',');
  const targetUrl = urls[__VU % urls.length].trim() + '?token=' + token;

  const pending = new Map();

  const res = ws.connect(targetUrl, {}, function (socket) {
    socket.on('open', function () {
      socket.setInterval(function () {
        const clientMessageId = uuidv4();
        pending.set(clientMessageId, Date.now());

        socket.send(JSON.stringify({
          conversationId: conversationID,
          clientMessageId: clientMessageId,
          body: 'Load test ping from VU ' + __VU,
        }));
        sentCounter.add(1);
      }, MSG_INTERVAL_MS);

      socket.setInterval(function () {
        socket.close();
      }, 50000); // close after 50s
    });

    socket.on('message', function (data) {
      recvCounter.add(1);
      try {
        const msg = JSON.parse(data);
        if (msg.clientMessageId && pending.has(msg.clientMessageId)) {
          roundtripTrend.add(Date.now() - pending.get(msg.clientMessageId));
          pending.delete(msg.clientMessageId);
        }
      } catch (err) {
        // ignore unparseable control frames
      }
    });

    socket.on('error', function () {
      errorCounter.add(1);
    });
  });

  check(res, { 'status is 101': (r) => r && r.status === 101 });
}