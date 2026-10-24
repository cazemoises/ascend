# HTTP laboratories validation — 2026-10-24

Recovered `0d30c1e` without rewriting or discarding its 34 files; recovery on the task branch is `650e3a2`.

## Go hang investigation

The earlier 14:09 execution's processes and logs were not available in this execution environment. No historical deadlock can be established from that report alone.

Go 1.26.2 (Linux arm64) is available directly. Dependencies include PostgreSQL, Redis, and the private rootless Podman CLI for integration tests. Initial commands spent time downloading modules and compiling a cold cache. The detailed `go test -x` trace showed compiler invocations before tests started. Every package's normal tests then completed successfully, run sequentially with `-p 1`, a Go test deadline, and `timeout -k 2s 58s` around each command. The Judge unit package completed in 6.2 seconds.

The first race-enabled HTTP sandbox command reached the external timeout before emitting any test-start event. One diagnostic retry with `-x` showed race compilation of dependencies (including Redis), then passed all scenarios in 13.2 seconds. This is evidence of cold compilation overhead, not a reproduced test deadlock.

Local diagnostic output is captured under `/tmp/ascend-http-diagnostic/` as individual text files. `timeout` terminates the command process group, and the background Judge is launched as a binary with a cleanup trap that sends a signal and waits for it. CI now follows the same sequential, bounded testing approach and launches the Judge binary directly instead of orphaning a `go run` child.

## Corrections

- An anonymous read of an unpublished lab now returns 404 instead of scanning SQL NULL into a bool and returning 500.
- The recovered router integration test supplies a real rate limiter (miniredis in its simulated mode); its previous nil limiter panicked on submission.
- HTTP grading suppresses hidden actual/expected responses and batch logs, which can contain hidden requests even when a sample fails first. Exploration and sample response feedback remain available. Regression tests cover hidden success/failure, sample failure, and exploration.
- Lab integration database operations and cleanup have deadlines.

## Validation evidence

- All ten Go packages enumerated across root/API/Judge/shared modules: pass (packages without tests compile successfully).
- Migration to v29: applied successfully against disposable PostgreSQL 16 in private rootless Podman.
- Mentor/student permissions and lab linking: pass, including anonymous access, ownership, teacher preview, immutable configuration, hidden cases, unchanged editorial state, and submission ownership.
- Real Redis → Judge → FastAPI sandbox flow: pass normally in 6.9 seconds and with race detection in 7.9 seconds. Exploration does not complete an item; graded acceptance does; wrong answers provide sample feedback; mentor runs do not become student attempts.
- Eight HTTP sandbox scenarios: pass normally (12.9 seconds) and with race detection (13.2 seconds): stateful methods/path/query/JSON, non-root UID, network isolation, status mismatch, redirects not followed, exploration, response budget, startup failure, deadline, and log budget. Sandbox removal is checked after each scenario.
- Existing dashboard/history/assessment integration checks: pass per API package with race detection. Existing executor lifecycle/isolation integration: pass with race detection (6.0 seconds).
- Frontend tests/lint/build were already validated in the recovered execution as reported by the owner; frontend files have no additional changes in this diagnostic follow-up. The official validation workflow reruns them.

Publication and deployment status must be obtained from the task PR and Actions runs at the current HEAD; this document does not assert that a merge or deploy occurred.
