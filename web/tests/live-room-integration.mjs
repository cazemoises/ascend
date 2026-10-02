import assert from 'node:assert/strict'
import * as Y from 'yjs'
import Socket from 'ws'
import { connectLiveRoom } from '../src/lib/liveRoomConnection.ts'

const base = process.argv[2]
const emails = process.env.LIVE_ROOM_EMAILS?.split(',')
if (!base) throw new Error('Usage: node live-room-integration.mjs <fixture websocket base URL>')
const clients = []
const delay = ms => new Promise(resolve => setTimeout(resolve, ms))
async function until(check, label) {
  const deadline = Date.now() + 15000
  while (!check()) {
    if (Date.now() > deadline) throw new Error(`Timed out: ${label}; texts=${JSON.stringify(clients.map(c => c.doc.getText('code').toString()))}`)
    await delay(20)
  }
}
function client(index) {
  const c = { doc: new Y.Doc(), connected: false, socket: undefined }
  c.connection = connectLiveRoom(c.doc, `${base}${index}`, value => { c.connected = value }, () => {}, url => {
    c.socket = emails ? new Socket(base, { headers: { 'Remote-Email': emails[index] } }) : new WebSocket(url)
    return c.socket
  })
  clients.push(c)
  return c
}
const contents = () => clients.map(c => c.doc.getText('code').toString())
const converged = () => new Set(contents()).size === 1
try {
  const a = client(0)
  const b = client(1)
  await until(() => a.connected && b.connected, 'initial hydration')
  a.doc.getText('code').insert(0, 'first\n')
  // No periodic snapshot is needed for a new participant to see these edits.
  a.doc.getText('code').insert(6, 'formerly-missing\n')
  const typing = (async () => {
    for (let i = 0; i < 80; i++) {
      for (const [c, prefix] of [[a, 'a'], [b, 'b']]) {
        const text = c.doc.getText('code')
        text.insert(Math.floor(text.length / 2), `${prefix}${i},`)
        if (i % 7 === 0 && text.length > 6) text.delete(2, 1)
      }
      await delay(3)
    }
  })()
  await delay(30)
  const late = client(2)
  await until(() => late.connected, 'join during active editing')
  late.doc.getText('code').insert(0, 'late-edit\n')
  await typing
  await until(converged, 'three simultaneous editors')
  assert.ok(contents()[0].length > 200)
  console.log('PASS: three editors, late join during active inserts/deletes')

  // Lose the connection immediately after an update, then edit while offline.
  b.doc.getText('code').insert(0, 'unacknowledged\n')
  b.socket.close()
  await until(() => !b.connected, 'disconnect')
  b.doc.getText('code').insert(0, 'offline\n')
  a.doc.getText('code').insert(0, 'while-disconnected\n')
  await until(() => b.connected, 'automatic reconnect')
  await until(converged, 'reconnect including offline edits')
  assert.ok(contents()[0].includes('offline'))
  assert.ok(contents()[0].includes('unacknowledged'))
  console.log('PASS: reconnect preserves offline and unacknowledged edits')

  // A fresh doc verifies the durable state independently of existing peers.
  const fresh = client(3)
  await until(() => fresh.connected && converged(), 'fresh hydration from durable journal')
  const union = new Y.Doc()
  for (const c of clients) Y.applyUpdate(union, Y.encodeStateAsUpdate(c.doc))
  for (const c of clients) {
    assert.equal(c.doc.getText('code').toString(), union.getText('code').toString())
    assert.deepEqual(Y.encodeStateVector(c.doc), Y.encodeStateVector(union))
  }
  union.destroy()
  console.log('PASS: four documents match content and Yjs state vectors')
} finally {
  for (const c of clients) { c.connection.close(); c.doc.destroy() }
}
