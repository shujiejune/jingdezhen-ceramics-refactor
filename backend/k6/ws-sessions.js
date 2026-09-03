// k6 WebSocket session load test (PRD §2.4.3 scenario 3).
//
// Ramps concurrent WS connections to find the Fiber WS + Redis pub/sub
// ceiling. Target: 500 concurrent WebSocket sessions (PRD §2.4.3 line 137).
// Each VU holds one WS connection open for the duration.
//
// Usage:
//   k6 run -e BASE_URL=ws://localhost:1323 -e TEST_TOKEN=<jwt> backend/k6/ws-sessions.js
import ws from 'k6/ws'
import { check } from 'k6'
import { Rate } from 'k6/metrics'

const BASE_URL = __ENV.BASE_URL || 'ws://localhost:1323'
const TEST_TOKEN = __ENV.TEST_TOKEN || ''

const errorRate = new Rate('errors')

export const thresholds = {
  errors: [{ threshold: 'rate<0.01', abort: false }], // WS: < 1% (looser than HTTP)
  ws_connecting: ['p(95)<1000'], // connect in < 1s
}

export const options = {
  scenarios: {
    ws_sessions: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 100 },  // ramp to 100
        { duration: '30s', target: 250 },  // ramp to 250
        { duration: '30s', target: 500 },  // ramp to 500 (PRD target)
        { duration: '1m', target: 500 },   // hold at 500
        { duration: '15s', target: 0 },    // ramp down
      ],
      gracefulRampDown: '15s',
    },
  },
  thresholds,
}

export default function () {
  const url = `${BASE_URL}/ws?token=${encodeURIComponent(TEST_TOKEN)}`

  ws.connect(url, {}, function (socket) {
    socket.on('open', () => {
      errorRate.add(false)
    })

    socket.on('message', () => {
      // The server may push notification frames; we just receive them
      // and keep the connection alive. No client-originated messages needed.
    })

    socket.on('error', () => {
      errorRate.add(true)
    })

    socket.on('close', () => {
      // Connection closed (by server timeout or load shedding)
    })

    // Keep the connection open for the duration of the VU's lifecycle.
    // k6 WS sessions auto-close when the VU iteration ends.
    socket.setTimeout(() => {
      socket.close()
    }, 60000) // 60s per iteration
  })
}
