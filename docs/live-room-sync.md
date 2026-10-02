# Collaborative room synchronization

All editors bind to the same `Y.Text("code")`. Cursor/selection remain local to
Monaco. The API stores opaque Yjs updates without interpreting the CRDT.

`live_room_updates` is the authoritative append-only journal. A transaction
increments the room's `doc_version` and inserts the update before broadcasting.
The row lock ensures a gap-free sequence in commit order. The migration copies
existing snapshots into an immutable `doc_baseline` before journaled editing
begins. State already lost before this migration cannot be reconstructed.

Connections register for broadcasts before reading the baseline and complete
journal in a single SQL statement snapshot. Duplicates and out-of-order delivery
are safe for Yjs. Each connection also replays journal entries after its last
replay frontier every second, repairing lost Redis pubsub events. All data
writes share a connection mutex and a write deadline. Freeze notifications
request final replay before the `4000` close frame.

The frontend retains its Y.Doc and Monaco binding across reconnect. After
hydration it sends its complete local state as a journaled update, recovering
offline edits and updates whose delivery was uncertain.

## Binary protocol

Integer fields are unsigned big endian; Yjs bytes use update encoding V1.

| Direction | Tag | Payload after tag |
| --- | --- | --- |
| Client → server | 3 | Yjs update (incremental or complete local state) |
| Server → client | 3 | uint64 journal version, then Yjs update |
| Server → client | 5 | uint64 fully replayed journal frontier |
| Client → server | 2 | uint64 frontier, uint32 Yjs length, Yjs snapshot, UTF-8 text |

Binary/text snapshot caches are saved atomically only when their frontier equals
the current `doc_version`. Unversioned legacy snapshot frames cannot overwrite
them. Hydration never trusts these caches or truncates the journal: a numeric
version alone does not prove that a client snapshot contains every dependency.
Journal storage is released when its session/room is deleted by FK cascade.
Deploying this protocol requires existing browser pages to reload.

## Validation

- `cd web && npm test`, `npm run lint`, `npm run build`.
- WSL Go: `go test ./cmd/... ./api/... ./judge/...`.
- Against a migrated disposable PostgreSQL database: set `DATABASE_URL`, then
  `cd api && go test -race -tags integration ./internal/handler ./internal/store`.
- The Yjs integration requires Node 24 plus `web` dependencies. Set
  `LIVE_ROOM_NODE` if Node is outside PATH (for example Windows Node from WSL).
  `REDIS_URL` enables real Redis; a second API deliberately misses pubsub so
  journal catch-up is exercised.
- `docker/live-room-tests.Dockerfile` builds a portable regression runner. The
  deploy workflow points `LIVE_ROOM_DEPLOYED_URL` at nginx's
  `/ascend/api/v1` mount and runs four actual Yjs clients against the deployed
  API. Its temporary fixture data is cleaned after the test.
