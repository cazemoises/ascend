# Public Test Cases (Sample Examples) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Embed sample (public) test cases in `GET /api/v1/challenges/:id` so the frontend can display them as "Exemplos de Entrada/Saída" on the challenge page, eliminating blind judging.

**Architecture:** The DB already has `is_sample BOOLEAN` on `test_cases` — no migration needed. A new store method `GetChallengeDetail` fetches the challenge and its sample cases in two sequential queries, returning a composite response. The handler and frontend types are updated to match. Private test case data (input/expected_output) is never exposed: the submission endpoint only returns the verdict string, and the new endpoint only returns rows where `is_sample = true`.

**Tech Stack:** Go (stdlib `database/sql`, `net/http`, chi), PostgreSQL, React + TypeScript (Vite, strict mode)

---

## Key Finding: No DB Migration Required

`test_cases.is_sample` (`BOOLEAN NOT NULL DEFAULT false`) already exists in `migrations/000001_initial_schema.up.sql` and is already read by the store's `ListTestCases` and `CreateTestCase` methods. The concept the user calls `is_public` maps 1-to-1 to `is_sample`. All work below is at the application layer only.

---

## File Map

| File | Change |
|------|--------|
| `api/internal/store/store.go` | Add `SampleTestCase`, `ChallengeDetail` types; add `GetChallengeDetail` method |
| `api/internal/store/store_test.go` | Add integration tests for `GetChallengeDetail` |
| `api/internal/handler/challenges.go` | Change `get` handler to call `GetChallengeDetail` |
| `web/src/api.ts` | Add `SampleTestCase` interface; extend `Challenge` with `sample_test_cases` |
| `web/src/pages/ChallengePage.tsx` | Render examples table below the description |

No other files touched.

---

### Task 1: Store — add `SampleTestCase`, `ChallengeDetail`, and `GetChallengeDetail`

**Files:**
- Modify: `api/internal/store/store.go`

- [ ] **Step 1: Write the failing integration test**

Add at the bottom of `api/internal/store/store_test.go` (inside the `//go:build integration` file):

```go
func TestGetChallengeDetail_SampleTestCases(t *testing.T) {
    db := openTestDB(t)
    s := store.New(db, nil)
    ctx := context.Background()

    ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
        Slug:       "detail-sample-test",
        Title:      "Detail Test",
        Difficulty: "easy",
    })
    if err != nil {
        t.Fatalf("CreateChallenge: %v", err)
    }
    t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

    // sample (public) test case
    _, err = s.CreateTestCase(ctx, ch.ID, store.CreateTestCaseRequest{
        Input:          "1 2",
        ExpectedOutput: "3",
        IsSample:       true,
    })
    if err != nil {
        t.Fatalf("CreateTestCase (sample): %v", err)
    }

    // private test case — must NOT appear in detail
    _, err = s.CreateTestCase(ctx, ch.ID, store.CreateTestCaseRequest{
        Input:          "secret input",
        ExpectedOutput: "secret output",
        IsSample:       false,
    })
    if err != nil {
        t.Fatalf("CreateTestCase (private): %v", err)
    }

    detail, err := s.GetChallengeDetail(ctx, ch.ID)
    if err != nil {
        t.Fatalf("GetChallengeDetail: %v", err)
    }
    if detail.ID != ch.ID {
        t.Errorf("challenge id: got %q, want %q", detail.ID, ch.ID)
    }
    if len(detail.SampleTestCases) != 1 {
        t.Fatalf("sample test cases: got %d, want 1", len(detail.SampleTestCases))
    }
    if detail.SampleTestCases[0].Input != "1 2" {
        t.Errorf("sample input: got %q, want %q", detail.SampleTestCases[0].Input, "1 2")
    }
    if detail.SampleTestCases[0].ExpectedOutput != "3" {
        t.Errorf("sample expected_output: got %q, want %q", detail.SampleTestCases[0].ExpectedOutput, "3")
    }
}

func TestGetChallengeDetail_NotFound(t *testing.T) {
    db := openTestDB(t)
    s := store.New(db, nil)

    _, err := s.GetChallengeDetail(context.Background(), "00000000-0000-0000-0000-000000000000")
    if err != store.ErrNotFound {
        t.Errorf("expected ErrNotFound, got %v", err)
    }
}

func TestGetChallengeDetail_EmptySamples(t *testing.T) {
    db := openTestDB(t)
    s := store.New(db, nil)
    ctx := context.Background()

    ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
        Slug:       "detail-no-samples",
        Title:      "No Samples",
        Difficulty: "hard",
    })
    if err != nil {
        t.Fatalf("CreateChallenge: %v", err)
    }
    t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

    detail, err := s.GetChallengeDetail(ctx, ch.ID)
    if err != nil {
        t.Fatalf("GetChallengeDetail: %v", err)
    }
    if detail.SampleTestCases == nil {
        t.Error("SampleTestCases must be an empty slice, not nil (JSON must serialize as [])")
    }
}
```

- [ ] **Step 2: Run test to verify it fails (method doesn't exist yet)**

```bash
go test -tags integration -run TestGetChallengeDetail ./api/internal/store/...
```

Expected: compile error — `s.GetChallengeDetail undefined`

- [ ] **Step 3: Add types and method to `api/internal/store/store.go`**

Insert after the `TestCase` struct (around line 48):

```go
type SampleTestCase struct {
    Input          string `json:"input"`
    ExpectedOutput string `json:"expected_output"`
    Ordinal        int    `json:"ordinal"`
}

type ChallengeDetail struct {
    Challenge
    SampleTestCases []SampleTestCase `json:"sample_test_cases"`
}
```

Add the method after `GetChallenge` (around line 146):

```go
func (s *Store) GetChallengeDetail(ctx context.Context, id string) (ChallengeDetail, error) {
    row := s.db.QueryRowContext(ctx,
        `SELECT `+challengeColumns+` FROM challenges WHERE id = $1`, id)
    c, err := scanChallenge(row)
    if errors.Is(err, sql.ErrNoRows) {
        return ChallengeDetail{}, ErrNotFound
    }
    if err != nil {
        return ChallengeDetail{}, err
    }

    rows, err := s.db.QueryContext(ctx,
        `SELECT input, expected_output, ordinal FROM test_cases
         WHERE challenge_id = $1 AND is_sample = true ORDER BY ordinal`, id)
    if err != nil {
        return ChallengeDetail{}, err
    }
    defer rows.Close()

    samples := make([]SampleTestCase, 0)
    for rows.Next() {
        var tc SampleTestCase
        if err := rows.Scan(&tc.Input, &tc.ExpectedOutput, &tc.Ordinal); err != nil {
            return ChallengeDetail{}, err
        }
        samples = append(samples, tc)
    }
    if err := rows.Err(); err != nil {
        return ChallengeDetail{}, err
    }

    return ChallengeDetail{Challenge: c, SampleTestCases: samples}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -tags integration -run TestGetChallengeDetail ./api/internal/store/...
```

Expected: all three `TestGetChallengeDetail_*` tests PASS

- [ ] **Step 5: Verify no existing tests broken**

```bash
go test ./api/...
```

Expected: PASS (unit tests; integration tests skipped without DATABASE_URL)

- [ ] **Step 6: Commit**

```bash
git add api/internal/store/store.go api/internal/store/store_test.go
git commit -m "feat(store): add GetChallengeDetail with sample test cases"
```

---

### Task 2: Handler — wire `GET /api/v1/challenges/:id` to `GetChallengeDetail`

**Files:**
- Modify: `api/internal/handler/challenges.go`

- [ ] **Step 1: Update the `get` method**

Replace the existing `get` method in `api/internal/handler/challenges.go`:

```go
func (h *ChallengesHandler) get(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    detail, err := h.store.GetChallengeDetail(r.Context(), id)
    if err != nil {
        if errors.Is(err, store.ErrNotFound) {
            writeError(w, http.StatusNotFound, "challenge not found")
            return
        }
        writeError(w, http.StatusInternalServerError, "internal server error")
        return
    }
    writeJSON(w, http.StatusOK, detail)
}
```

(The only change is `GetChallenge` → `GetChallengeDetail` and `c` → `detail`.)

- [ ] **Step 2: Compile check**

```bash
go build ./api/...
```

Expected: no errors

- [ ] **Step 3: Run existing unit tests to confirm nothing broken**

```bash
go test ./api/...
```

Expected: PASS (existing handler tests don't cover GET /{id} path — they test validation only)

- [ ] **Step 4: Smoke test (requires running server + seeded DB)**

```bash
# From WSL/bash
curl -s http://localhost:8080/api/v1/challenges/<CHALLENGE_ID> | jq '.sample_test_cases'
```

Expected: `[]` (empty array, not `null`) if no sample test cases; or array of objects with `input`, `expected_output`, `ordinal` if samples exist.

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/challenges.go
git commit -m "feat(api): embed sample_test_cases in GET /challenges/:id response"
```

---

### Task 3: Frontend — update `api.ts` types

**Files:**
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add `SampleTestCase` interface and extend `Challenge`**

At the top of `web/src/api.ts`, after the existing `ChallengeDifficulty` and `SubmissionLanguage` exports, add:

```typescript
export interface SampleTestCase {
  input: string
  expected_output: string
  ordinal: number
}
```

Then update the `Challenge` interface to add `sample_test_cases`:

```typescript
export interface Challenge {
  id: string
  slug: string
  title: string
  description: string
  difficulty: ChallengeDifficulty
  time_limit_ms: number
  memory_limit_mb: number
  sample_test_cases: SampleTestCase[]
  created_at: string
  updated_at: string
}
```

- [ ] **Step 2: TypeScript compile check**

```bash
cd web && npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add web/src/api.ts
git commit -m "feat(web/api): add SampleTestCase type; add sample_test_cases to Challenge"
```

---

### Task 4: Frontend — render examples table in `ChallengePage.tsx`

**Files:**
- Modify: `web/src/pages/ChallengePage.tsx`

- [ ] **Step 1: Add the examples section inside the challenge panel**

In `ChallengePage.tsx`, add the examples block immediately after the metadata `<div>` (the `challenge-card__meta` div, which ends around line 110) and before the `<form>`:

```tsx
{challenge.sample_test_cases.length > 0 ? (
  <div style={{ marginBottom: '24px' }}>
    <h2 style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '12px' }}>
      Exemplos
    </h2>
    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
      <thead>
        <tr>
          <th style={{ textAlign: 'left', padding: '6px 12px', borderBottom: '1px solid var(--border)' }}>
            Entrada
          </th>
          <th style={{ textAlign: 'left', padding: '6px 12px', borderBottom: '1px solid var(--border)' }}>
            Saída Esperada
          </th>
        </tr>
      </thead>
      <tbody>
        {challenge.sample_test_cases.map((tc, i) => (
          <tr key={i}>
            <td style={{ padding: '6px 12px', verticalAlign: 'top' }}>
              <pre style={{ margin: 0, fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>
                {tc.input || '(vazio)'}
              </pre>
            </td>
            <td style={{ padding: '6px 12px', verticalAlign: 'top' }}>
              <pre style={{ margin: 0, fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>
                {tc.expected_output}
              </pre>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  </div>
) : null}
```

- [ ] **Step 2: TypeScript compile check**

```bash
cd web && npx tsc --noEmit
```

Expected: no errors (the `SampleTestCase` type added in Task 3 covers the `tc` variable)

- [ ] **Step 3: Visual verification (requires running dev server)**

```bash
cd web && npm run dev
```

Navigate to a challenge that has at least one test case with `is_sample = true`. The "Exemplos" table should appear between the metadata badges and the submission form.

To seed a sample test case for manual verification:
```bash
# From WSL/bash
curl -s -X POST http://localhost:8080/api/v1/challenges/<CHALLENGE_ID>/test-cases \
  -H "Content-Type: application/json" \
  -d '{"input":"1 2","expected_output":"3","is_sample":true}' | jq
```

Then reload the challenge page — the table should appear with `1 2` / `3`.

- [ ] **Step 4: Verify a challenge without sample test cases**

Navigate to a challenge with no sample test cases. The "Exemplos" section should not appear at all (clean layout, no empty table).

- [ ] **Step 5: Run linter and tests**

```bash
cd web && npm run lint && npm test
```

Expected: no lint errors, tests pass

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/ChallengePage.tsx
git commit -m "feat(web): show sample test cases as examples table on challenge page"
```

---

## Security Confirmation (No Code Changes Needed)

The submission result endpoint (`GET /api/v1/submissions/:id`) already only returns `id, challenge_id, language, source_code, status, created_at, updated_at` — it never exposes test case data. The worker stores only a verdict string (`accepted`, `wrong_answer`, `time_limit_exceeded`, `runtime_error`). Private test cases flow only through the in-process worker → Docker container loop, never touching the HTTP response path. No changes required.

---

## Self-Review Checklist

**Spec coverage:**
- [x] `is_sample` column already exists — no migration needed (noted explicitly)
- [x] Store method filters by `is_sample = true` — private data never in response
- [x] `GET /api/v1/challenges/:id` embeds sample test cases
- [x] Frontend displays examples table
- [x] Private test case inputs/outputs never leak via submissions endpoint (already secure)

**Placeholder scan:** None found — all tasks contain complete code.

**Type consistency:**
- `SampleTestCase` defined in Task 1 (Go store), Task 3 (TS) — fields consistent: `input`, `expected_output`, `ordinal`
- `ChallengeDetail.SampleTestCases` in Go matches `Challenge.sample_test_cases` in TS (snake_case JSON)
- `GetChallengeDetail` defined in Task 1, called in Task 2 — signature matches
