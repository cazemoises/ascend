# Ascend

Ascend is an online code judge for programming challenges, with tools for
mentoring and collaboration. A React/TypeScript frontend sends submissions to
a Go API; Redis queues them for a Go judge worker that executes solutions in
isolated sandboxes and stores results in PostgreSQL.

## Tests

Run from the repository root in a Linux shell (WSL Ubuntu on Windows), with
Go 1.26 or newer and Node.js/npm available:

```bash
go test ./...
go test ./api/...
go test ./judge/...
npm --prefix web ci
npm --prefix web test
```

The Go workspace (`go.work`) contains four modules: the root, `api`, `judge`,
and `pkg` (which currently has no Go packages). Testing the root alone does not
include the nested modules.
The frontend test script in `web/package.json` uses Node's built-in test runner.
These commands run the default tests without the `integration` build tag;
they do not require a database or running sandbox containers.
