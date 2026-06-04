## 1. Database Migration

- [x] 1.1 Criar `migrations/000004_add_challenge_notes.up.sql` com `ALTER TABLE challenges ADD COLUMN notes TEXT;`
- [x] 1.2 Criar `migrations/000004_add_challenge_notes.down.sql` com `ALTER TABLE challenges DROP COLUMN notes;`
- [x] 1.3 Rodar a migration via docker: `docker run --rm -v $(pwd):/src -w /src --network ascend_default -e DATABASE_URL=postgres://ascend:ascend@postgres:5432/ascend?sslmode=disable golang:1.26-alpine go run ./cmd/migrate up`

## 2. Store Layer — campo notes

- [x] 2.1 Em `api/internal/store/store.go`, adicionar `Notes *string \`json:"notes"\`` à struct `Challenge` (após `MemoryLimitMb`)
- [x] 2.2 Atualizar a constante `challengeColumns` para incluir `notes` (adicionar `, c.notes` na query)
- [x] 2.3 Atualizar `scanChallenge` para fazer Scan de `&c.Notes` (na posição correspondente ao novo campo)
- [x] 2.4 Adicionar `Notes *string` ao struct `CreateChallengeRequest`
- [x] 2.5 Atualizar a query INSERT em `CreateChallenge` para incluir a coluna `notes` e o parâmetro `$N` correspondente

## 3. Store Layer — histórico de submissões

- [x] 3.1 Em `api/internal/store/store.go`, adicionar o tipo `SubmissionSummary` com campos `ID`, `Status`, `Language`, `CreatedAt` (json tags: `id`, `status`, `language`, `created_at`)
- [x] 3.2 Adicionar método `ListRecentSubmissions(ctx context.Context, challengeID string, limit int) ([]SubmissionSummary, error)` que executa `SELECT id, status, language, created_at FROM submissions WHERE challenge_id = $1 ORDER BY created_at DESC LIMIT $2` e retorna `make([]SubmissionSummary, 0)` se vazio; retorna `ErrNotFound` se o challenge não existir (verificar com `GetChallenge` antes, ou tratar via LEFT JOIN — use a query simples com verificação prévia)

## 4. API Handler — notas e novo endpoint

- [x] 4.1 Em `api/internal/handler/challenges.go`, adicionar `Notes *string \`json:"notes"\`` ao struct `createChallengeBody`
- [x] 4.2 No handler `create`, passar `Notes: body.Notes` no `store.CreateChallengeRequest`
- [x] 4.3 Adicionar handler `listSubmissions(w http.ResponseWriter, r *http.Request)` em `challenges.go` que chama `h.store.ListRecentSubmissions(r.Context(), id, 10)` e retorna JSON 200; retorna 404 se `store.ErrNotFound`
- [x] 4.4 Em `ChallengesHandler.Routes`, registrar `r.Get("/{id}/submissions", h.listSubmissions)`
- [x] 4.5 Rodar `go vet ./api/...` (from WSL) e corrigir quaisquer erros

## 5. Frontend — api.ts

- [x] 5.1 Adicionar `notes: string | null` à interface `Challenge` em `web/src/api.ts`
- [x] 5.2 Adicionar interface `SubmissionSummary` com campos `id: string`, `status: string`, `language: SubmissionLanguage`, `created_at: string`
- [x] 5.3 Adicionar função `listChallengeSubmissions(challengeId: string): Promise<SubmissionSummary[]>` que chama `GET /api/v1/challenges/:id/submissions`

## 6. Frontend — ChallengePage

- [x] 6.1 Criar (ou adicionar inline no ChallengePage) um objeto `CODE_TEMPLATES` com chave `SubmissionLanguage` e templates de starter code
- [x] 6.2 Em `ChallengePage.tsx`, adicionar estado `submissions: SubmissionSummary[]` (inicializar com `[]`) e um `useEffect` paralelo que chama `listChallengeSubmissions(id)` e atualiza o estado; erros de network não bloqueiam o restante da página
- [x] 6.3 Adicionar lógica de template: no `onChange` do `<select>` de linguagem, após `setLanguage(...)`, verificar `if (sourceCode === '') { setSourceCode(CODE_TEMPLATES[newLanguage]) }`
- [x] 6.4 Renderizar o bloco de `notes` abaixo de `<p className="muted">{challenge.description}</p>`: se `challenge.notes`, renderizar `<p style={{ ...estilo destacado... }}>{challenge.notes}</p>` antes da tabela de exemplos
- [x] 6.5 Renderizar painel de histórico abaixo do `<button>Submit</button>` (dentro do `<form>` ou após): se `submissions.length > 0`, tabela com colunas "Status", "Linguagem", "Data"; se `submissions.length === 0`, `<p className="muted">Sem submissões ainda.</p>`

## 7. Type check e smoke tests

- [x] 7.1 Rodar `npx tsc --noEmit` em `web/` e corrigir erros de tipo
- [x] 7.2 Rodar `npm run lint` em `web/` e corrigir warnings
- [x] 7.3 Smoke test (WSL curl): `POST /api/v1/challenges` com `"notes": "Leia dois inteiros e imprima a soma."` — verificar `notes` no response
- [x] 7.4 Smoke test (WSL curl): `GET /api/v1/challenges/:id` — verificar que `notes` e `sample_test_cases` aparecem
- [x] 7.5 Smoke test (WSL curl): `GET /api/v1/challenges/:id/submissions` — verificar array (mesmo vazio) retornado com 200
- [ ] 7.6 Verificação manual no browser: navegar ao desafio, confirmar bloco de notes visível, confirmar template aplicado ao mudar linguagem com editor vazio, confirmar painel de histórico
