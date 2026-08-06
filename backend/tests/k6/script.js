import ws from 'k6/ws';
import { check } from 'k6';
import { Trend, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const roundtripTrend = new Trend('ws_roundtrip_latency_ms');
const errorCounter = new Counter('ws_errors');
const sentCounter = new Counter('ws_messages_sent');
const recvCounter = new Counter('ws_messages_received');

// One entry per seeded user: { userID, token, conversationIDs: [...] }.
const userPool = new SharedArray('user_pool', function () {
  return JSON.parse(open('./tokens.json'));
});

const MSG_INTERVAL_MS = 1000;
const CONNECTION_LIFETIME_MS = 50000;

// Default to nginx, the single ingress that load-balances the realtime tier.
// Override with -e URL=ws://host:port/ws to hit a different endpoint.
const URL = __ENV.URL || 'ws://localhost:8000/ws';

export const options = {
  thresholds: {
    ws_roundtrip_latency_ms: ['p(95)<150', 'p(99)<300'],
    ws_errors: ['count<100'],
  },
};

export default function () {
  // Each VU acts as one seeded user. With more VUs than users the pool wraps,
  // so several VUs can share a user — a realistic multi-device scenario.
  const user = userPool[(__VU - 1) % userPool.length];
  const token = user.token;
  const conversationIDs = user.conversationIDs;

  const targetUrl = URL + '?token=' + token;

  const pending = new Map();

  const res = ws.connect(targetUrl, {}, function (socket) {
    socket.on('open', function () {
      socket.setInterval(function () {
        // Spread traffic across every conversation this user belongs to.
        const conversationID =
          conversationIDs[Math.floor(Math.random() * conversationIDs.length)];

        const clientMessageId = uuidv4();
        pending.set(clientMessageId, Date.now());

        socket.send(JSON.stringify({
          conversationId: conversationID,
          clientMessageId: clientMessageId,
          body: 'Load test ping from VU ' + __VU,
        }));
        sentCounter.add(1);
      }, MSG_INTERVAL_MS);

      socket.setTimeout(function () {
        socket.close();
      }, CONNECTION_LIFETIME_MS);
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
