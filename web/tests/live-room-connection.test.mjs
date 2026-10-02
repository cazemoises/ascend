import assert from 'node:assert/strict'
import test from 'node:test'
import * as Y from 'yjs'
import { connectLiveRoom, snapshotFrame } from '../src/lib/liveRoomConnection.ts'

function serverFrame(tag, version, update = new Uint8Array()) {
  const bytes = new Uint8Array(9 + update.length)
  bytes[0] = tag
  new DataView(bytes.buffer).setBigUint64(1, BigInt(version))
  bytes.set(update, 9)
  return bytes.buffer
}

class FakeSocket {
  readyState = WebSocket.OPEN
  sent = []
  send(data) { this.sent.push(data) }
  receive(tag, version, update) { this.onmessage({ data: serverFrame(tag, version, update) }) }
  close() {
    this.readyState = WebSocket.CLOSED
    this.onclose?.({ code: 1006 })
  }
}

test('reconnect replays complete state and resends offline and unacknowledged edits', async () => {
  const local = new Y.Doc()
  const remote = new Y.Doc()
  const sockets = []
  const connection = connectLiveRoom(local, 'ws://room', () => {}, () => {}, () => {
    const socket = new FakeSocket()
    sockets.push(socket)
    return socket
  })
  try {
    sockets[0].receive(5, 0)
    local.getText('code').insert(0, 'unacknowledged\n')
    sockets[0].close()
    local.getText('code').insert(local.getText('code').length, 'offline\n')
    remote.getText('code').insert(0, 'remote\n')
    await new Promise(resolve => setTimeout(resolve, 550))
    assert.equal(sockets.length, 2)
    sockets[1].receive(3, 1, Y.encodeStateAsUpdate(remote))
    sockets[1].receive(5, 1)
    const resend = sockets[1].sent[0]
    assert.equal(resend[0], 3)
    Y.applyUpdate(remote, resend.subarray(1))
    assert.equal(remote.getText('code').toString(), local.getText('code').toString())
    assert.ok(remote.getText('code').toString().includes('offline'))
    assert.ok(remote.getText('code').toString().includes('unacknowledged'))
  } finally { connection.close(); local.destroy(); remote.destroy() }
})

test('out-of-order and duplicate updates converge before saving the replay frontier', () => {
  const source = new Y.Doc()
  const updates = []
  source.on('update', update => updates.push(update))
  source.getText('code').insert(0, 'dependency\n')
  source.getText('code').insert(11, 'dependent\n')
  const local = new Y.Doc()
  const socket = new FakeSocket()
  const connection = connectLiveRoom(local, 'ws://room', () => {}, () => {}, () => socket)
  try {
    socket.receive(3, 2, updates[1])
    socket.receive(3, 1, updates[0])
    socket.receive(3, 2, updates[1])
    socket.receive(5, 2)
    assert.equal(local.getText('code').toString(), source.getText('code').toString())
    connection.snapshot()
    const saved = socket.sent.at(-1)
    assert.equal(saved[0], 2)
    assert.equal(new DataView(saved.buffer).getBigUint64(1), 2n)
    assert.deepEqual(saved, snapshotFrame(local, 2n))
  } finally { connection.close(); local.destroy(); source.destroy() }
})

test('freeze stops reconnect and control frames are never applied as Yjs updates', async () => {
  const doc = new Y.Doc()
  let frozen = false
  let opens = 0
  const socket = new FakeSocket()
  const connection = connectLiveRoom(doc, 'ws://room', () => {}, () => { frozen = true }, () => { opens++; return socket })
  try {
    socket.receive(5, 0)
    socket.onclose({ code: 4000 })
    await new Promise(resolve => setTimeout(resolve, 550))
    assert.equal(frozen, true)
    assert.equal(opens, 1)
    assert.equal(doc.getText('code').toString(), '')
  } finally { connection.close(); doc.destroy() }
})
