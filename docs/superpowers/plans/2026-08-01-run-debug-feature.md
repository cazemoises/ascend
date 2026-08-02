# "Executar" (Run/Debug) Feature — Architecture Investigation

> **Status: investigation only, not approved.** Per this session's task
> queue, item 11 is the one item that requires explicit sign-off before any
> implementation — everything below is proposal + open questions for review,
> not a plan to execute unattended.

**Goal (as given):** let a student run their code against the challenge's
sample input, see raw stdout (and stderr, presumably), without it counting
as a real submission attempt — a fast iteration loop for adding debug
`print`s, distinct from "Enviar solução".

---

## What already exists (relevant findings)

- **`is_test_run` is not reusable for this.** It exists (migration
  `000013_submission_test_run`) but is derived server-side from the JWT role
  (`claims.Role == "teacher"`, see `api/internal/handler/submissions.go:70`)
  — it means "a teacher testing their own challenge," not "any user doing a
  throwaway run." Reusing it here would conflate two different concepts and
  give students a way to flip a flag that currently only means "teacher."
  A "Run" feature needs its own, separate marker if it goes through the
  submissions table at all (see Option A below).

- **The API container has no Docker access.** `docker-compose.yml` mounts
  `/var/run/docker.sock` only into the `judge` service, not `api`. This is a
  real security boundary (the API is the more exposed, JWT-authenticated
  surface; the judge worker is not directly reachable). Any design that
  needs the API to invoke Docker directly (for a fast synchronous
  request/response) would either widen that boundary or need its own
  justification — flagging this because it's exactly the kind of
  "architectural convention already closed" this session was told to stop
  for, not decide around unilaterally.

- **The existing flow is queue-and-poll, not request/response.** `POST
  .../submissions` returns `202` with just an id; the judge worker `BLPOP`s
  the job, executes, writes the verdict to Postgres; the frontend polls
  `GET /submissions/:id` every 2s until `status != pending`
  (`SubmissionPage.tsx`, `POLLING_DELAY_MS = 2000`). A debug-loop feature
  wants to feel fast (a student iterating on prints many times a minute) —
  a 2s+ poll cadence is a real UX cost worth deciding on explicitly, not
  inheriting by default.

- **Rate limiting is already a reusable, generic component.**
  `middleware.RateLimiter` takes a Redis client, a limit/window, and a
  `KeyFunc` — the existing submission limiter is just one instantiation
  (`ratelimit:submissions:<user>`, 10/60s). A second instance with a
  different key prefix and its own limit/window is cheap to add and would
  not touch the existing submission limiter's behavior.

- **Fail-fast and multi-test-case evaluation don't obviously apply.** The
  worker's `evaluate()` stops at the first failing test case across *all*
  of a challenge's test cases (sample + hidden). A "Run" only ever needs
  the sample input(s) — hidden test case content must never reach a Run
  response, same withholding rule `failed_input`/`failed_is_sample` already
  enforce for `wrong_answer` results.

---

## Open questions (need your call, not mine)

1. **What input does Run execute against?**
   - Just the challenge's first/only sample test case, or every sample if
     there's more than one?
   - Or does the student get a free-text input box to paste *their own*
     input, closer to what "debugging" usually means (not just re-running
     the canned example)? This changes both the UI and the request payload
     shape non-trivially, so it needs to be decided up front, not
     discovered mid-implementation.

2. **Where do results live — Postgres or ephemeral Redis?**
   - **Option A: reuse the submissions table.** Add an `is_run BOOLEAN`
     column (sibling to `is_test_run`), same queue/worker/poll machinery,
     excluded from `submission history`/stats the same way `is_test_run`
     already is. Least new code, but means every debug run becomes a
     permanent row — could be a lot of rows for a feature meant to be run
     many times per session, and "excluded from stats" has to be threaded
     through every query that already excludes `is_test_run` (three
     separate `WHERE ... is_test_run = false` clauses in
     `api/internal/store/store.go` alone, plus `teacher_overview.go`) — a
     second flag to keep in sync with the first everywhere, forever.
   - **Option B: a separate, ephemeral store.** A new Redis job type +
     result written to a `run:<id>` key with a short TTL (e.g. 5 minutes)
     instead of a Postgres row. No migration, nothing to exclude from
     stats because it was never in the stats tables to begin with, and it
     self-cleans. Costs a second (small) result-shape/polling code path
     alongside the existing submission one instead of one shared path.
   - My inclination is **B** — it keeps "real attempt" and "debug run" as
     genuinely separate concepts end to end instead of one flag deep inside
     a shared table, and avoids unbounded growth of a table that already
     backs student-facing history — but this is exactly the kind of
     tradeoff worth your explicit call, not mine.

3. **New endpoint or new field on the existing one?**
   Given (2), a new endpoint (`POST /challenges/:id/run`, separate from
   `POST /challenges/:id/submissions`) seems clearly right regardless of
   A vs B above — it keeps `CreateSubmission`'s validation/response shape
   untouched and makes the two concepts distinguishable at the routing
   layer, not just by a body field a client could get wrong.

4. **Rate limit policy for Run.** Reuse the existing 10/60s submission
   limiter's bucket, or give Run its own (probably more generous, since
   the whole point is rapid iteration)? Leaning toward a separate,
   looser bucket via a second `RateLimiter` instance, but the actual
   numbers are a product call.

5. **Does the worker need a genuinely separate job type**, or can it stay
   one `SubmissionJob`-shaped queue with a `kind` field the worker
   branches on? Leaning toward one queue, branching on `kind`, to avoid
   running two separate `BLPOP` loops in the same worker process — but
   this only matters once (2) is settled, since it's downstream of
   whether there's a Postgres row to look up at all.

---

## Sketch of Option B (Redis-ephemeral) + free-text input, IF that's the
direction — not a commitment, just to make the tradeoffs concrete:

| Layer | Change |
|---|---|
| `web/src/pages/ChallengePage.tsx` | New "Executar" button next to "Enviar solução"; optional input textarea if question 1 goes the free-text way |
| `api/internal/handler` | New `POST /api/v1/challenges/:id/run` — same JWT/language/source_code validation as `CreateSubmission`, no `is_test_run` derivation, no DB write; enqueues a `{kind: "run", ...}` job and returns `{run_id}` |
| `api/internal/middleware` | Second `RateLimiter` instance, own bucket/limit/window |
| `judge/internal/worker/worker.go` | Job dispatch branches on `kind`; a `run` job fetches only the challenge's sample test case(s) (never hidden ones), executes via the existing `DockerExecutor`, writes `{stdout, stderr, exit_code}` to `run:<id>` in Redis with a TTL instead of updating `submissions` |
| `api/internal/handler` | New `GET /api/v1/runs/:id` — polls the Redis key, same 404-until-ready shape `GET /submissions/:id` already has for `pending` |

Rough sizing if this shape is approved: 1 migration-free backend change
(no schema touched), 1 new frontend polling hook mirroring
`SubmissionPage.tsx`'s existing one, no changes to the fail-fast/verdict
machinery test submissions already use.

---

## What I did NOT do

Per the instructions for this item specifically: no code was written, no
endpoint added, no migration drafted. This file is the entire deliverable
for item 11 — next step is your review and a decision on the open
questions above before any implementation starts.
