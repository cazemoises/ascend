# LSP-Grade Java Autocomplete — Feasibility Investigation (Not Implemented)

**Decision: not recommended now — same conclusion as
[2026-08-01-lsp-autocomplete-evaluation.md](2026-08-01-lsp-autocomplete-evaluation.md),
reinforced by concrete findings below rather than re-derived from scratch.**
A much cheaper alternative exists that actually answers the literal request
("`String` suggests `.length()`, `.substring()`") — see Recommendation.

This reopens that prior decision at explicit request, scoped to Java only,
for real member-access completion (type-aware, not just syntax snippets).
Investigation only — nothing in this document has been implemented pending
review.

---

## Task 1 — Technical findings

### 1. jdtls Docker availability
Eclipse JDT Language Server (jdtls) is the standard Java LSP implementation,
confirmed via [the project's GitHub](https://github.com/eclipse-jdtls/eclipse.jdt.ls).
**There is no official Eclipse-published Docker image.** What exists is
community-maintained, e.g.
[Kaylebor/eclipse.jdt.ls-docker](https://github.com/Kaylebor/eclipse.jdt.ls-docker)
(built from source, published to Docker Hub under a personal namespace).
Adopting a third-party image for a process that stays alive and keeps
parsing untrusted student input is a meaningfully different trust decision
than the per-language images this judge already runs (`python:3.11-alpine`,
`golang:1.26-alpine`, `eclipse-temurin:21-jdk-alpine`, etc. — all official).
The realistic path is building and maintaining our own jdtls image, same
category of effort as `docker/cpp.Dockerfile` / `docker/sqlite.Dockerfile`
already in this repo, except those are static and jdtls would need to track
upstream releases indefinitely.

One piece of good news: jdtls requires a **Java 21 runtime minimum**, and
the judge already runs Java submissions on `eclipse-temurin:21-jdk-alpine`
(`judge/internal/worker/executor.go`) — no new base runtime, just a new
image layered on a JDK we already depend on.

### 2. monaco-languageclient bridge
Confirmed necessary: Monaco doesn't speak LSP natively, so
`monaco-languageclient` (TypeFox) + a WebSocket transport is the standard
bridge, exactly as the prior evaluation described.

Confirmed **compatible** with this project's actual installed version —
this was checked against real numbers, not assumed:
- `web/package.json` pins `@monaco-editor/react@^4.7.0`, whose peer range
  resolves `monaco-editor` to `>= 0.25.0 < 1`
  (`web/package-lock.json:570`).
- The resolved version in `web/node_modules/monaco-editor/package.json` is
  **0.55.1**.
- Per
  [monaco-languageclient's versions-and-history doc](https://github.com/TypeFox/monaco-languageclient/blob/main/docs/versions-and-history.md),
  monaco-languageclient **10.7.0 (2026-02-04) through 10.8.0** is aligned
  with monaco-editor 0.55.1.

So compatibility is a non-issue if this is ever built. That's the one part
of the original evaluation's uncertainty this investigation actually closes.

### 3. Execution model
Confirmed fundamentally different from the judge's resource model. The
judge's Java path is one-shot: `docker run` per submission, dies the moment
`javac`/`java` finish (`judge/internal/worker/executor.go`). A language
server needs to stay alive for the duration of a student's editing session
to give live completions.

Two shapes, both real options, both with real costs:
- **Shared single instance** serving every student: rejected on inspection,
  not just cost. jdtls doesn't cleanly support multiple truly-isolated
  workspaces in one process — completions/diagnostics from one student's
  in-progress (possibly broken) code could leak into another's session.
  That's a correctness bug, not just an efficiency one.
- **One process per active editing session**: correct isolation, but N
  processes running concurrently, each paying jdtls's real memory cost (see
  Task 2).

### 4. Security
Confirmed this is a different risk class from the existing submission
sandbox, not just "the same sandbox running longer." Today's Java
submissions run with `--network none`, a memory cap, and exit within one
`time_limit_ms` window — engineered to be disposable. A live jdtls process
would need to keep parsing whatever a student keeps typing for as long as
their tab is open: same isolation primitives (no network, capped memory,
non-root) are necessary but not sufficient — the process also has to
survive an entire session without leaking resources or accumulating state
across reconnects, which nothing in the current judge design was built to
guarantee (it was explicitly built to *not* need to survive past one run).

---

## Task 2 — Sizing

### New components required
Not a patch to the existing judge — closer to standing up a second,
differently-shaped subsystem alongside it:
1. A jdtls Docker image to build and keep updated (no official one to lean
   on — see Task 1.1).
2. A WebSocket gateway: new backend surface to route each browser session's
   edits to the right jdtls process and back, including session→process
   routing, auth (would need to reuse `PangolinAuth`'s identity, since
   there's no existing per-session backend concept to hang this off today),
   and idle-session reaping.
3. A process-lifecycle manager: spawn-on-demand, health-check, kill-on-idle
   or kill-on-disconnect — genuinely new, the judge worker has no analog to
   this today (`BLPOP` → run → done is its entire lifecycle model).
4. Frontend wiring: `monaco-languageclient` integration in
   `ChallengePage.tsx` alongside the existing `registerAscendSnippets` call.

That's three new backend components plus frontend work, not a small
addition.

### Memory
Real numbers, not a guess:
- A [jdtls GitHub issue](https://github.com/eclipse-jdtls/eclipse.jdt.ls/issues/745)
  reports the server using 300MB+ even when capped at `-Xmx64m` — it
  doesn't respect a very low ceiling well. General Eclipse guidance is to
  not go below `-Xmx512M`, with 1.5GB as the default for normal workspaces.
  Since jdtls indexes the JDK standard-library classpath on startup
  regardless of project size, a single 20-line student file doesn't
  meaningfully shrink this cost — expect **300–500MB+ per concurrently
  active Java-editing session**, realistically, not per submission.

**Correction on the stated memory premise:** the task described "a VM que
já teve problema de RAM essa sessão, 444MB total." I checked this against
actual current numbers rather than build on it as given:
- Windows host: `Get-CimInstance Win32_ComputerSystem` reports
  **~16.8GB** total physical memory.
- Docker Desktop's VM: `docker info --format '{{.MemTotal}}'` reports
  **8,152,313,856 bytes (~8.15GB)** allocated to Docker.
- Current combined usage of postgres+redis+api+judge, idle: under 60MB
  total (`docker stats --no-stream`).

I don't have a source for "444MB total" in this environment as it stands
right now — flagging the discrepancy rather than quietly building a
recommendation on an unverified number. That said, **the conclusion doesn't
change even using the real 8.15GB figure**: idle infra costs under 60MB
today; jdtls at 300–500MB per active session means roughly 15+ students
with the Java tab open concurrently would already approach the entire
Docker VM's memory budget, before postgres/redis/api/judge's own headroom
under real submission load is accounted for. jdtls would be, by a wide
margin, the single heaviest process class in the stack.

### Cheaper MVP considered
On-demand-per-session-with-idle-timeout (spawn when a student opens the
Java editor, kill after N minutes idle) bounds concurrency to "students
actively editing Java right now" instead of "students who ever will," which
is strictly better than always-on — but it's a smaller version of the same
three components above, not a different-in-kind cost. It still needs the
WebSocket gateway and process-lifecycle manager; it only removes the
"always running" tax, not the "new backend subsystem" tax. It also adds a
real UX cost the always-on version wouldn't have: cold-start latency after
each idle-kill, since jdtls's classpath indexing on startup takes several
seconds, not something a student should notice mid-typing.

---

## A genuinely cheaper alternative (not full LSP)

The prior decision's implemented "básico tier" — Monaco defaults +
`web/src/lib/monacoSnippets.ts`'s `registerCompletionItemProvider` — is the
right extension point for a middle ground that was not evaluated in the
original doc because it wasn't in scope then: **client-side curated
member-completion**, no backend process at all.

Concretely: extend the same completion provider with a small static map of
the JDK types this judge's Java challenges actually use —
`String`, `Scanner`, `List`/`ArrayList`, `Map`/`HashMap`, `LinkedList`,
`StringBuilder` (this is literally the full set that appears across the 41
`trilha-*` Java harnesses just regenerated in
`scripts/fix_trilha_java_class_structure.sql`) — to their common member
methods. A lightweight heuristic (regex-match `TypeName varName =`
declarations in the buffer, remember `varName`'s declared type, offer that
type's member list after `varName.`) covers the exact example given in the
task (`String` → `.length()`, `.substring()`) without real type inference.

This doesn't do cross-file resolution, generic inference, or anything a
real language server does — but neither does this judge's use case need
that (unchanged from the prior doc's core argument: every challenge here is
one self-contained file). It's zero new runtime processes, zero new
security surface, and effort proportionate to "a few hours," matching the
cost class the prior decision already accepted rather than the one it
rejected.

---

## Recommendation

**Full jdtls-based LSP: not recommended now.** The prior decision's
reasoning holds, and this investigation adds concrete numbers that make the
cost side heavier than assumed, not lighter: no official Docker image to
build on, a genuinely new 3-component backend subsystem, and 300–500MB+ per
concurrently active session with no cheap way around the classpath-indexing
cost. The trigger condition for revisiting is unchanged from the prior doc:
multi-file submissions or a genuine IDE mode outside the judge flow — scope
hasn't changed, so the calculus hasn't either.

**What I'd actually propose instead:** the client-side curated
member-completion extension described above. It directly answers the
literal request in this task ("real method autocomplete for common types
like `String`") at the same cost tier as the already-shipped snippet
provider. I have not implemented this or anything else — flagging it as
the concrete next step to approve, separately from the full-LSP question
this document was asked to resolve.

Sources:
- [eclipse-jdtls/eclipse.jdt.ls](https://github.com/eclipse-jdtls/eclipse.jdt.ls)
- [Kaylebor/eclipse.jdt.ls-docker](https://github.com/Kaylebor/eclipse.jdt.ls-docker)
- [monaco-languageclient versions-and-history](https://github.com/TypeFox/monaco-languageclient/blob/main/docs/versions-and-history.md)
- [monaco-languageclient on npm](https://www.npmjs.com/package/monaco-languageclient)
- [eclipse-jdtls/eclipse.jdt.ls issue #745 — memory usage exceeds Xmx](https://github.com/eclipse-jdtls/eclipse.jdt.ls/issues/745)
