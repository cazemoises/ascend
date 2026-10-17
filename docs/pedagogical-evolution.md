# Evolução pedagógica — entregas incrementais

## Etapa 1: indicadores no progresso dos alunos

Implementada em 17/10/2026; integração PostgreSQL validada, publicação em preparação.

A rota existente `GET /api/v1/teacher/students-overview`, restrita a professores,
passa a incluir:

- `stats.unresolved_challenges`: desafios tentados sem nenhuma aceitação.
- `stats.recurring_difficulties`: desafios sem aceitação com pelo menos três
  falhas conhecidas (wrong_answer, runtime_error ou time_limit_exceeded).
- Totais históricos em `stats.wrong_answer_count`, `runtime_error_count` e
  `timeout_count`, inclusive dos desafios posteriormente resolvidos.
- `weekly_progress`: semanas com envios, ordenadas por segunda-feira em UTC,
  com `week_start`, `attempts` e `newly_solved`. Cada desafio conta como nova
  resolução apenas na semana da primeira aceitação. Semanas sem envios não
  aparecem. Tentativas incluem pendentes; pendentes não contam como falhas.

A página de progresso mostra esses indicadores, os desafios que precisam de
atenção, um filtro de alunos com dificuldades recorrentes e uma tabela semanal.
O limite de três falhas é uma sinalização para o mentor, não um diagnóstico.
A contagem é histórica e não indica inatividade ou dificuldade recente.
Alunos que só concluíram itens de listas continuam visíveis com evolução vazia.

Sem migration: todos os valores são derivados das submissões existentes.
Envios de teste e contas que não são de aluno continuam excluídos pela consulta.
A rota, permissões, código fonte dos envios, sandbox e protocolo Yjs não mudam.
A ordenação das tentativas usa data e ID para desempatar datas iguais.

### Validação

- `go test ./cmd/... ./api/... ./judge/...` passou no ambiente Linux Go 1.26.2.
- `go test -race ./api/internal/store` passou.
- Novos testes unitários cobrem limiar de dificuldade, falhas combinadas,
  resolução posterior, pendentes, semanas UTC, virada de ano, primeira
  aceitação e ausência de atividade.
- PostgreSQL 16 descartável em Podman rootless, migrado até 000027:
  `go test -race -tags integration ./api/internal/store ./api/internal/handler ./api/internal/router -run 'TestTeacherStudentsOverview|TestStudentsOverview|TestPedagogicalDashboard'`
  passou sem skips nos testes de dashboard. O teste de deploy exige URL explícita.
- Router real validado com identidade Pangolin: mentor 200, aluno 403,
  visitante 401, aluno tentando elevar papel 403, mentor em visualização de
  aluno 403. Verificados JSON semanal, exclusão de test runs, aluno sem
  atividade e remoção do alerta após aceitação.
- Smoke HTTP contra API em execução passou: `TestPedagogicalDashboardDeployed`,
  usando `LIVE_ROOM_DEPLOYED_URL` no banco descartável. Incluído no runner da
  pipeline oficial para repetir a verificação após publicação, com fixtures
  únicas e limpeza em ordem de FK. Não imprime dados de alunos reais.
- Regressão integrada Yjs passou com PostgreSQL/Redis privados: três editores,
  entrada tardia, reconexão offline e quatro documentos convergentes.
- Novo workflow `Validate Ascend` verifica Go, integração PostgreSQL rootless,
  frontend, lint e build em PRs para `main` antes do deploy existente.
- Frontend: `cd web && npm test` passou (regressões de reconexão Yjs);
  `npm run lint` e `npm run build` passaram.
- Validação manual pendente: entrar como professor e conferir aluno sem envios,
  aluno com três falhas, filtro, aceitação posterior e tabela em tela estreita;
  confirmar que aluno não acessa a rota do professor.

Commit, push, PR e merge desta tarefa foram autorizados em 17/10/2026.
A publicação continua exclusivamente pela pipeline oficial, após os checks
do HEAD do PR. O journal da sessão pertence ao worker.

## Próximas entregas e dependências

1. Histórico de sessões: listagem explícita de sessões encerradas, restrita ao
   proprietário/participante, com relatório final somente de leitura. Hoje a
   consulta exclui encerradas; os dados e o dashboard já existem. Não montar
   controles de rodadas ao consultar histórico. Independente desta etapa.
2. Exercícios objetivos: múltipla escolha e resposta curta, com modelo de
   respostas/versões e tentativas persistidas antes de expor correção automática.
   Separar respostas corretas dos dados públicos; reutilizar papéis e listas.
   SQL já existe: testar e evoluir o fluxo atual, evitando um segundo executor.
3. Trilhas: módulos sobre listas, pré-requisitos validados sem ciclos, liberação
   verificada na API e progresso por aluno. Depende de um contrato de conclusão
   consistente para exercícios de código e objetivos; não basta ocultar na UI.
4. Dashboard ampliado: evolução por módulo e tipo de exercício após etapas 2–3;
   atualização ao vivo deve reutilizar eventos existentes sem expor dados de
   outros alunos. Considerar paginação/agregação SQL quando o volume justificar:
   o endpoint atual carrega todo o histórico de tentativas.
5. Editor/judge: validar recuperação e reconexão Yjs conforme
   `docs/live-room-sync.md`; evoluir feedback e limites em sandbox rootless
   privado. Exercícios de APIs exigem primeiro um executor de testes HTTP
   isolado, sem acesso à rede externa, com limites de recursos e tempo. Nunca
   executar código não confiável no processo da API nem no Docker de produção.

Cada etapa deve fechar backend, UI e testes antes da seguinte. Preservar dados
com migrations append-only quando houver mudança de schema. Não há aprovação
implícita para publicar exercícios, alterar situação editorial ou fazer deploy.
