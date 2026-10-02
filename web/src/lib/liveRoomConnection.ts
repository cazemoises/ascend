import * as Y from 'yjs'

const UPDATE = 3
const SNAPSHOT = 2
const READY = 5
const REMOTE = Symbol('live-room-remote')

export function updateFrame(update: Uint8Array): Uint8Array<ArrayBuffer> {
  const frame = new Uint8Array(update.length + 1)
  frame[0] = UPDATE
  frame.set(update, 1)
  return frame
}

export function snapshotFrame(doc: Y.Doc, version: bigint): Uint8Array<ArrayBuffer> {
  const update = Y.encodeStateAsUpdate(doc)
  const text = new TextEncoder().encode(doc.getText('code').toString())
  const frame = new Uint8Array(13 + update.length + text.length)
  frame[0] = SNAPSHOT
  const view = new DataView(frame.buffer)
  view.setBigUint64(1, version)
  view.setUint32(9, update.length)
  frame.set(update, 13)
  frame.set(text, 13 + update.length)
  return frame
}

// Owns transport only: the same Y.Doc (and Monaco binding) survives reconnect.
// Journal replay, not a periodic client snapshot, establishes the frontier.
export function connectLiveRoom(
  doc: Y.Doc,
  url: string,
  onConnected: (connected: boolean) => void,
  onFrozen: () => void,
  createSocket: (url: string) => WebSocket = (address) => new WebSocket(address),
) {
  let socket: WebSocket | undefined
  let stopped = false
  let ready = false
  let version = 0n
  let retry: ReturnType<typeof setTimeout> | undefined
  let attempts = 0
  const sendSnapshot = () => {
    if (ready && socket?.readyState === WebSocket.OPEN) socket.send(snapshotFrame(doc, version))
  }
  const onUpdate = (update: Uint8Array, origin: unknown) => {
    if (origin !== REMOTE && ready && socket?.readyState === WebSocket.OPEN) {
      socket.send(updateFrame(update))
    }
    // Offline/unacknowledged operations remain in doc and are resent on READY.
  }
  doc.on('update', onUpdate)
  const interval = setInterval(sendSnapshot, 5000)

  const connect = () => {
    if (stopped) return
    ready = false
    version = 0n
    const current = createSocket(url)
    socket = current
    current.binaryType = 'arraybuffer'
    current.onmessage = (event) => {
      if (stopped || socket !== current || !(event.data instanceof ArrayBuffer)) return
      const data = new Uint8Array(event.data)
      if (data.length < 9) return
      if (data[0] === UPDATE) {
        if (data.length > 9) {
          try { Y.applyUpdate(doc, data.subarray(9), REMOTE) }
          catch { current.close(); return }
        }
      } else if (data[0] === READY) {
        version = new DataView(event.data).getBigUint64(1)
        if (!ready) {
          ready = true
          attempts = 0
          // Full local state is itself persisted as a Yjs update. This covers
          // edits made offline and updates sent immediately before a disconnect.
          current.send(updateFrame(Y.encodeStateAsUpdate(doc)))
          onConnected(true)
        }
      }
    }
    current.onclose = (event) => {
      if (stopped || socket !== current) return
      ready = false
      onConnected(false)
      if (event.code === 4000) {
        stopped = true
        clearInterval(interval)
        onFrozen()
        return
      }
      retry = setTimeout(connect, Math.min(500 * 2 ** attempts++, 5000))
    }
    current.onerror = () => current.close()
  }
  connect()
  return {
    snapshot: sendSnapshot,
    close: () => {
      sendSnapshot()
      stopped = true
      clearInterval(interval)
      if (retry !== undefined) clearTimeout(retry)
      doc.off('update', onUpdate)
      socket?.close()
    },
  }
}
