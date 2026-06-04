## Context

Ascend é um juiz online com backend Go (chi router, `database/sql`, PostgreSQL) e frontend React + TypeScript com Vite. A página do desafio (`/challenges/:id`) atualmente exibe título, descrição, badges de dificuldade/tempo/memória, tabela de exemplos (`is_sample=true`) e o Monaco Editor controlado. Não há templates de código, não há campo de instrução livre e não há histórico de tentativas visível ao usuário. As três melhorias são independentes entre si mas tocam as mesmas camadas (DB → store → handler → frontend).

## Goals / Non-Goals

**Goals:**
- Adicionar coluna `notes TEXT NULL` na tabela `challenges` via migration append-only
- Expor `GET /api/v1/challenges/:id/submissions` retornando as últimas 10 submissões (summary-only)
- Exibir `notes`, histórico de submissões e templates de código na `ChallengePage`

**Non-Goals:**
- Filtro por usuário no histórico (auth não implementado)
- Paginação do histórico (limitado a 10, suficiente para esta fase)
- Templates editáveis pelo admin (hardcoded no frontend)
- Monaco readOnly mode para o histórico (somente exibição textual)

## Decisions

### D1: Migration 000004 — `ALTER TABLE ADD COLUMN notes TEXT`

`notes` é opcional e nullable, logo `ALTER TABLE challenges ADD COLUMN notes TEXT` sem default é seguro em tabela populada (todas as linhas existentes ficam `NULL`). Sem downtime porque PostgreSQL não reescreve linhas para colunas nullable sem default. Migration de rollback: `ALTER TABLE challenges DROP COLUMN notes`.

**Alternativa descartada:** campo separado `challenge_instructions` — complexidade desnecessária; `notes` basta como texto livre.

### D2: Endpoint `GET /challenges/:id/submissions` no handler de challenges

O endpoint é montado em `ChallengesHandler.Routes` como `r.Get("/{id}/submissions", h.listSubmissions)`. O store precisa de um novo método `ListRecentSubmissions(ctx, challengeID string, limit int) ([]SubmissionSummary, error)` — retorna apenas `id`, `status`, `language`, `created_at` (sem `source_code`).

**Alternativa descartada:** handler separado `SubmissionsHandler` — exige refatorar o roteador; o endpoint é logicamente pertencente ao recurso `challenges`.

### D3: `Challenge` struct ganha `Notes *string`

Ponteiro para distinguir `null` (não informado) de `""` (string vazia) na serialização JSON. O campo Go é `Notes *string \`json:"notes"\``. `scanChallenge` passa `&c.Notes` no `Scan`. `createChallengeBody` e `CreateChallengeRequest` aceitam `*string`.

**Alternativa descartada:** `sql.NullString` — mais verboso; `*string` com `omitempty` não é adequado aqui (queremos `null` explícito no JSON), então usamos `*string` sem `omitempty`.

### D4: Templates de código hardcoded no frontend

Três strings constantes em `ChallengePage.tsx` (ou arquivo `templates.ts` separado). Ao trocar a linguagem, se `sourceCode === ''`, aplica o template via `setSourceCode(template[language])`. Sem API, sem persistência.

**Alternativa descartada:** templates no banco por linguagem — over-engineering; o conteúdo é estável e raramente muda.

### D5: Histórico carregado em `useEffect` paralelo ao `getChallenge`

`ChallengePage` dispara duas chamadas paralelas: `getChallenge(id)` e `listChallengeSubmissions(id)`. O histórico tem seu próprio estado `submissions` e é renderizado abaixo do editor independentemente do estado de loading do desafio.

## Risks / Trade-offs

- **Sem user_id no histórico** → o painel mostra submissões de todos os usuários (incluindo futuras). Aceitável enquanto não houver auth; o campo será filtrado depois.
- **`notes` como texto livre** → sem markdown rendering; exibido em `<p>` com `white-space: pre-wrap`. Suficiente por ora.
- **Limite fixo de 10** → hardcoded no store. Se precisar de paginação, basta expor `limit`/`offset` como query params no futuro.

## Migration Plan

1. `docker compose up -d` (postgres já rodando)
2. Criar `migrations/000004_add_challenge_notes.up.sql` e `.down.sql`
3. Rodar `go run ./cmd/migrate up` via docker
4. Rebuildar a API: `docker compose build api && docker compose up -d api`
5. Rollback: `go run ./cmd/migrate down` (executa o `.down.sql`)
