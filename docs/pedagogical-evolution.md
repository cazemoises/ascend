# Evolução pedagógica — entregas incrementais

## Etapa 1: indicadores no progresso dos alunos

Implementada e validada em 17/10/2026. Commit de implementação: `528e49e`.
PR: https://github.com/cazemoises/ascend/pull/1, mesclado como `14636f4`.
Publicado pela pipeline https://github.com/cazemoises/ascend/actions/runs/39046697343.
CI passou no HEAD `82eec09` antes do merge. Deploy e validação de `main`
passaram; o log de pós-deploy confirmou `TestPedagogicalDashboardDeployed`
e convergência Yjs (entrada tardia, reconexão e quatro documentos).

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
- Interface conferida com Chromium/Playwright em container privado rootless,
  frontend e API reais sobre fixtures locais: mentor vê aluno com três falhas,
  filtro funciona, tabela semanal mostra primeira resolução, aceitação remove
  alerta, tela de 375px não transborda, aluno recebe bloqueio e ViewAs bloqueia
  o painel/restaura acesso ao voltar a professor. O primeiro ensaio direto
  entre origens esbarrou no CORS do header de identidade; o ensaio válido usa
  proxy de mesma origem, como a implantação. Nenhum ajuste de CORS foi feito.
  Aluno sem envios e aluno só com conclusões de listas cobertos em PostgreSQL.

Commit, push, PR e merge desta tarefa foram autorizados em 17/10/2026.
A publicação continua exclusivamente pela pipeline oficial, após os checks
do HEAD do PR. O journal da sessão pertence ao worker.

## Etapa 2: histórico e resultados das sessões

Implementação validada localmente: `cbb3c6a`.
PR: https://github.com/cazemoises/ascend/pull/2. Checks e publicação são
acompanhados no PR e nas execuções vinculadas ao commit de merge.
O primeiro check detectou uma corrida na inicialização do banco descartável;
a sondagem agora aguarda TCP, evitando o servidor temporário de socket Unix.

- `GET /api/v1/live-sessions?status=finished`: somente sessões próprias do
  mentor ou sessões em que o aluno participou. O padrão mantém as sessões
  abertas; `status=open` é explícito e outros valores retornam 400.
- `GET /api/v1/live-sessions/{id}/results`: exige sessão encerrada e acesso
  como mentor proprietário efetivo ou participante. Sessão aberta retorna
  409 ao proprietário; visitante recebe 401 e estranho recebe 404.
- Mentor recebe tentativas individuais de todos os participantes; aluno
  recebe apenas suas próprias tentativas. Test runs e submissões fora da
  sessão ficam excluídos. Resumos não retornam código, snapshots ou testes.
- Resultados coletivos aparecem separadamente, com veredito e contagem de
  testes. Uma sala nunca iniciada é exibida sem criar registros. Não se
  atribui aceitação coletiva a cada aluno. Corrigida também a consulta do
  dashboard existente para excluir submissões de sala sem autor individual.
- Aba Histórico na tela de sessões e relatório para os três modos. A tela
  encerrada não monta painéis ativos, abre sockets ou configura rodadas.
  Rodadas ainda ativas no registro são descritas como interrompidas, sem
  expirar ou reescrever os dados. Links de prática não levam `liveSessionId`.
- Avaliações pendentes mantêm atualização do relatório até o judge concluir.
  `preparing_results` cobre o intervalo entre encerramento, congelamento e
  criação da submissão coletiva; salas congeladas vazias não geram espera.
  Falha persistente na preparação exige intervenção do mentor/operador, sem
  tentar reavaliar ou corrigir dados através da consulta histórica.
  Troca de identidade/papel não reaproveita resultados de outro contexto.

Sem migration; usa os registros persistidos. Títulos, itens e enunciados
refletem a versão atual da lista, sem snapshot curricular histórico. Listagem
sem paginação por enquanto, como a listagem aberta existente.

Validação: `TestLiveSessionHistory` passou com PostgreSQL privado e detector
`-race` nos três modos e contra API local real via `LIVE_ROOM_DEPLOYED_URL`.
Cobre visibilidade da lista, isolamento por aluno, erros de acesso, pendentes,
resultado coletivo e igualdade dos registros de salas, rodadas e journal
antes/depois das leituras. O contrato integra o check de PR e o smoke oficial
após deploy, com fixtures únicas e limpeza. Chromium conferiu os três modos,
acesso de mentor/aluno, links de prática, sala não iniciada e rodada
interrompida. Monitoramento da navegação confirmou zero escritas na API e
zero sockets de sessão ao consultar o arquivo. Navegação para UUID inexistente
mostra erro em vez de carregamento permanente. Fixtures locais descartadas
em ordem de FK; a API preserva listas referenciadas por sessões (409).

## Próximas entregas e dependências

1. Concluir validação e publicação da etapa 2 antes de iniciar exercícios novos.
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
