# Language-Server-Grade Autocomplete — Evaluation (Not Implemented)

**Decision: not worth it right now.** This file documents why, so the
question doesn't get re-litigated from scratch next time it comes up.

---

## What "básico" tier already covers (implemented this session)

- `quickSuggestions`/`wordBasedSuggestions` were already Monaco defaults
  (`{other:'on'}` / `'matchingDocuments'`) — pinned explicitly in
  `ChallengePage.tsx`'s Editor `options` so a future Monaco upgrade can't
  silently change them.
- Curated syntax snippets (for/if/while/function-or-def/main skeletons, plus
  a couple of idiomatic one-liners like Go's `err != nil` check or Java's
  `sout`) registered per language via
  `monaco.languages.registerCompletionItemProvider` in
  `web/src/lib/monacoSnippets.ts`. Confirmed by reading monaco-editor's own
  `basic-languages/*` source that none of python/go/java/c/cpp/javascript
  shipped any completion provider at all before this — only tokenizers for
  syntax highlighting.

Both are native Monaco APIs, zero new infrastructure, zero new runtime
dependencies.

## What "real" (LSP-grade) autocomplete would mean

Type-aware completion (member access on an actual inferred type, real
go-to-definition, cross-file symbol resolution, inline type errors) needs an
actual language server per language — pyright/pylsp (Python), gopls (Go),
clangd (C/C++), jdtls (Java) — plus a client-side bridge, since Monaco
doesn't speak LSP natively (`monaco-languageclient` + a WebSocket transport
is the standard shape). Concretely, this would require:

1. A long-lived language-server **process per active editing session**, one
   per language a student happens to be using — a fundamentally different
   resource model from the judge's current one-shot `docker run` per
   submission (see `judge/internal/worker/executor.go`), which exits the
   moment a submission finishes.
2. A WebSocket gateway routing each browser session's edits to the right
   server process and back — new backend surface, new deployment unit, new
   thing to keep running and healthy alongside api/judge/postgres/redis.
3. Four-plus language servers to install, configure, and keep working
   (pyright, gopls, clangd, jdtls at minimum — Node/JS gets something close
   to this for free from Monaco's bundled TypeScript service already, so
   it's the one language where the gap between "básico" and "real" is
   smallest).
4. A real security surface: unlike a submission's sandboxed, network-isolated,
   `--pids-limit`-capped, single-shot container, a language server is a
   process that stays alive and keeps parsing whatever the student keeps
   typing for as long as the tab is open.

## Why that's not worth it here

This is an exercise judge, not an IDE for open-ended projects. Every
challenge here is a single self-contained file reading stdin and writing
stdout — no multi-file navigation, no dependency graph, no project-wide
refactors, nothing a full language server's actual differentiators (cross-
file go-to-definition, project-wide rename, type inference across modules)
would ever get exercised on. The realistic ceiling of value for this
specific use case is closer to "decent autocomplete in a single 30-line
file" than "IDE-grade tooling," and word-based suggestions + curated
snippets already cover most of that ceiling for effort that's actually
proportionate — a few hours of native config vs. standing up and running a
whole new backend service class.

If this gets revisited, the trigger condition to watch for is scope change,
not time passing: if challenges start allowing multi-file submissions, or
the platform adds a genuine "IDE mode" outside the judge flow, the
calculus above changes and LSP becomes worth a real look. Absent that,
re-evaluating this on a fixed schedule isn't useful — nothing about the
cost/benefit here decays with time on its own.
