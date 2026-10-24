# Experiência pedagógica dos laboratórios HTTP

## Antes e depois

| Antes | Depois |
| --- | --- |
| Objetivo e contrato técnico na abertura | Contexto → problema → missão → exemplo → orientação → contrato → conclusão |
| Requisição e resposta em blocos extensos | Cards com método, rota, query, headers, corpo e status; representação técnica expansível |
| Cliente sem explicar sua relação com o editor | Explica main.py → servidor → pedido → resposta; botão recarrega exemplos públicos |
| Estado inicial pedia executar ou enviar | Orienta localizar editor, experimentar, comparar e submeter |
| Veredito exigia conhecer a plataforma | Próximos passos para diferenças, erro, timeout e aceitação; exploração nunca promete aprovação |

Reutilizados: ChallengePage, ReactMarkdown, CodeEditor/Monaco, HTTPWorkspacePanel,
HTTPExamples, SubmissionResult, HTTPTestDebugger e classes sample/result-panel.
Nenhuma mudança de arquitetura ou avaliação. Cards e orientações dependem dos
exemplos públicos e do tipo FastAPI, sem IDs ou comportamentos exclusivos dos
seis itens. httpDebuggerView continua protegendo dados de casos ocultos.

## Atualização administrativa controlada

O pacote `api/internal/handler/activity_templates/fastapi-v1.json` permanece
intacto. `fastapi-pedagogy-v2.json` no mesmo diretório altera somente items[].body.
Os testes comparam toda a configuração e a preservação literal dos contratos.
Para novas listas de teste, importe v2 pelo fluxo normal. O populador antigo
não serve para atualizar: ele publica.

Produção exige **operação administrativa explicitamente validada**, separada
do deploy. Esta entrega prepara o plano; não executa mudanças em produção.

```sh
python3 scripts/plan_http_lab_pedagogy.py \
  --list-id UUID_DA_LISTA --owner-id UUID_DO_PROFESSOR \
  --output /CAMINHO/PRIVADO/revisao-http.sql
```

O gerador não conecta ao banco. Cria arquivo exclusivo modo 0600, contendo
testes originais: revise privadamente, nunca entregue ao aluno ou anexe a
relatórios públicos. Após validação administrativa explícita, o operador aplica
com a conexão legítima do projeto e `psql -X -v ON_ERROR_STOP=1 -f ARQUIVO`.
Não colocar credenciais no comando ou relatório.

A transação verifica lista, proprietário, seis títulos únicos, vínculos,
dificuldades, enunciado original ou revisado, linguagem, limites, código inicial
e todos os testes em ordem. Divergência reverte tudo. Bloqueios de tabela
impedem edição concorrente durante a operação; lock timeout 5s e statement
timeout 30s limitam espera. Não há retry automático. Altera somente
list_items.body e challenges.description. Repetição não grava novamente.
Não importa, duplica, exclui, publica nem muda submissões, IDs, timestamps,
progresso, testes ou configuração.

## Testar os seis laboratórios

Na lista **Laboratórios HTTP · Primeiros passos com FastAPI**, abra **resolver
desafio**. Os IDs existentes determinam `/ascend/challenges/:id`; este documento
não presume IDs ou URLs de produção.

| Laboratório | Primeiro experimento |
| --- | --- |
| 01 · GET | GET /health: status 200 e identificação da biblioteca |
| 02 · POST | POST /echo: enviar text e comparar texto e length |
| 03 · Parâmetros | Comparar GET /books/7 e /books/7?lang=en |
| 04 · Validação | POST /loans: comparar days 1 e 0, criação e erro 422 |
| 05 · CRUD | Lote de criação, leitura, alteração e exclusão de livros |
| 06 · Reservas | Consultar estoque, reservar, exceder saldo e conferir que a falha o preserva |

Em cada item: leia a missão, complete **main.py**, abra **Cliente HTTP**, carregue
os exemplos e execute. Compare status e JSON em **Resultados**. Consulte a
diferença pública ou logs, ajuste e repita. **Submeter para avaliação** registra
conclusão após aceitação. Explorar conclui apenas a execução, não a atividade.

## Validação

`web/tests/http-labs-browser.mjs` importa v2 e verifica narrativa, cards e
carregamento de exemplos, além dos seis itens, JSON inválido, exploração real,
falha pública, correção no Monaco, submissão e conclusão. Captura desktop e
celular. Exige localhost descartável, banco novo, API, Redis e Judge; preparação
em `.github/workflows/validate.yml`. Use timeout externo de 300s.

Após a jornada, execute somente no banco descartável rootless:

```sh
ASCEND_PEDAGOGY_TEST_DB=NOME_CONTAINER_DESCARTAVEL \
  timeout -k 2s 58s python3 -m unittest discover -s scripts -p 'test_*http_lab*.py'
```

O teste integrado exercita v1 → v2, repetição, rollback por divergência e
igualdade de testes, submissões, situação editorial e demais campos. Não executa
sem container descartável explícito. A pipeline repete os checks antes do merge.

Validação local desta entrega: dez pacotes Go passaram com deadline por pacote;
14 testes frontend, lint e build passaram. Chromium real percorreu os seis
laboratórios com API/Redis/Judge e sandbox FastAPI privados: exploração, erro,
depurador, correção Monaco, avaliação e seis conclusões. Desktop 1440px e celular
390px preservaram a interface. Sete testes administrativos passaram sem skips
no PostgreSQL 16 descartável, incluindo atualização, idempotência e rollback.
Como o host não tem libatk, o ensaio usou servidor Playwright v1.64.1 em container
rootless privado, via ASCEND_HTTP_BROWSER_WS e encaminhamento de loopback; CI
usa Chromium local normalmente. Nada disso comprova atualização de produção.
