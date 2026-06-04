## Why

A página do desafio é o coração da experiência do usuário, mas hoje ela oferece pouco contexto e nenhum feedback histórico: não há instruções de leitura de stdin, o editor começa vazio, e o usuário não sabe se já tentou o desafio antes. Essas três melhorias atacam os pontos de maior fricção antes de qualquer autenticação.

## What Changes

- **Histórico de submissões**: novo endpoint `GET /api/v1/challenges/:id/submissions` retorna as últimas 10 submissões do desafio (globais, sem filtro de usuário por ora) com `id`, `status`, `language`, `created_at`. Exibido na página do desafio como tabela abaixo do editor.
- **Campo `notes`**: coluna `TEXT` opcional na tabela `challenges`. Retornado nos endpoints de detalhe e lista. Exibido abaixo da descrição como bloco de instrução destacado (ex: "Leia dois inteiros A e B do stdin, imprima a soma.").
- **Templates de código por linguagem**: ao selecionar python/go/javascript no `<select>`, o editor Monaco pré-popula com um template mínimo (leitura de stdin + estrutura básica) **somente se o editor estiver vazio**. Troca de linguagem com código já digitado não sobrescreve.

## Capabilities

### New Capabilities

- `challenge-submission-history`: listagem das últimas submissões de um desafio via API + exibição na página do desafio

### Modified Capabilities

- `challenges-crud`: adição do campo `notes` (TEXT, nullable) — novo requirement de criação/detalhe/listagem
- `database-schema`: nova coluna `notes` na tabela `challenges` via migration append-only
- `code-editor`: requirement de template inicial por linguagem quando editor está vazio
- `web-frontend`: renderização de `notes` abaixo da descrição e painel de histórico de submissões abaixo do editor

## Impact

- **DB**: nova migration `000004_add_challenge_notes.up.sql` com `ALTER TABLE challenges ADD COLUMN notes TEXT`
- **API store**: `Challenge` struct ganha campo `Notes *string`; `CreateChallengeRequest` e `createChallengeBody` ganham `notes`
- **API handler**: `GET /api/v1/challenges/:id/submissions` — novo handler em `challenges.go`
- **Frontend**: `web/src/api.ts` — `Challenge` + novo tipo `SubmissionSummary` + nova função `listChallengeSubmissions`; `ChallengePage.tsx` — template logic + painel de histórico + renderização de notes
