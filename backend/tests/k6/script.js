import ws from 'k6/ws';
import { check } from 'k6';
import { Trend, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const roundtripTrend = new Trend('ws_roundtrip_latency_ms');
const errorCounter = new Counter('ws_errors');
const sentCounter = new Counter('ws_messages_sent');
const recvCounter = new Counter('ws_messages_received');
const timeoutCounter = new Counter('ws_message_timeouts');

const tokenPool = new SharedArray('token_pool', function () {
  return JSON.parse(open('./tokens.json'));
});

const VUS = parseInt(__ENV.VUS || '50');
const DURATION = __ENV.DURATION || '10s';
const TARGET_MSG_RATE = parseFloat(__ENV.TARGET_MSG_RATE || '1.0') // per second
const MSG_INTERVAL_MS = Math.round((VUS / TARGET_MSG_RATE) * 1000)
const MSG_TIMEOUT_MS = parseInt(__ENV.MSG_TIMEOUT_MS || '10000');
const WS_URLS = __ENV.WS_URLS || 'ws://localhost:8080/ws';

export const options = {
  scenarios: {
    ws: {
      executor: 'constant-vus',
      vus: VUS,
      duration: DURATION,
    },
  },
  thresholds: {
    ws_roundtrip_latency_ms: ['p(95)<500', 'p(99)<1000'],
    ws_errors: ['count<1'],
    ws_message_timeouts: ['count<1'],
  },
};

export default function () {
  const userObj = tokenPool[__VU % tokenPool.length];
  const token = userObj.token;
  const conversationID = userObj.conversationID || '00000000-0000-0000-0000-000000000001';

  const urls = WS_URLS.split(',');
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
        const now = Date.now();
        for (const [id, sentAt] of pending) {
          if (now - sentAt > MSG_TIMEOUT_MS) {
            timeoutCounter.add(1);
            pending.delete(id);
          }
        }
      }, Math.max(MSG_INTERVAL_MS, 2000));

      socket.setInterval(function () {
        socket.close();
      }, 10000);
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
        // Ignore unparseable control frames
      }
    });

    socket.on('error', function (e) {
      errorCounter.add(1);
      console.error(`WS error VU=${__VU} url=${targetUrl} error=${e.error()}`);
    });
  });

  check(res, { 'status is 101': (r) => r && r.status === 101 });
}