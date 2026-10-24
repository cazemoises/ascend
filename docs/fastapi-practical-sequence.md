# Laboratórios HTTP no fluxo normal do Ascend

Professores cadastram atividades em **listas → nova lista → Importar JSON**.
O contrato oficial `POST /api/v1/lists/import` aceita `items[].body` (enunciado
Markdown) e `items[].http_lab` com `starter_code`, `time_limit_ms`,
`memory_limit_mb` e `test_cases`. Cada teste contém `input` e `expected_output`
como strings JSON, além de `is_sample`. A importação persiste lista, itens,
desafios e testes em uma transação. A lista nasce em rascunho; a publicação
continua na página da lista. Não há tela nem botão de geração FastAPI.

Para itens de uma lista existente, **laboratórios HTTP** permite fornecer
código inicial, limites e testes. `POST /lists/:id/items` também aceita
`http_lab` atomicamente. Configurações já criadas são imutáveis; novas versões
exigem novos itens, preservando submissões e progresso existentes.

## População administrativa inicial

O pacote `api/internal/handler/activity_templates/fastapi-v1.json` preserva as
seis atividades **Laboratórios HTTP · Primeiros passos com FastAPI**: GET,
POST/Pydantic, parâmetros, validação, CRUD e erros de domínio/estoque. O pacote
contém testes ocultos e não é servido pela API nem incluído no bundle do aluno.

Execute uma única operação administrativa com a sessão legítima de professor
fornecida pelo operador (cookie jar Netscape, se esse for o mecanismo suportado
pelo gateway), ou por um encaminhamento local já autenticado:

```sh
python3 scripts/populate_http_labs.py \
  --base-url https://DOMINIO/ascend \
  --cookies /CAMINHO/PRIVADO/session.cookies \
  --checkpoint /CAMINHO/PERSISTENTE/ascend-http-population.json
```

Esse comando **cadastra e publica** a lista pela API oficial. Não altera usuários,
listas tradicionais nem atividades preexistentes. Antes de publicar, confere
os seis vínculos, enunciados, configurações e testes persistidos. Uma lista com
mesmo título só é reutilizada se pertencer à sessão e coincidir com o pacote;
divergências ou múltiplos resultados interrompem a operação. Não execute
operadores simultâneos com checkpoints diferentes. Guarde o checkpoint: ele
registra a tentativa antes do POST e impede repetir uma importação cuja
resposta se perdeu. Não há retry automático de escrita. Ao retomar, a operação
consulta o inventário antes de decidir qualquer ação. Não injete identidade
Pangolin em produção. O comando imprime título, quantidade e URL somente após
confirmar publicação e leitura pública da lista.

## Validação reproduzível

`web/tests/http-labs-browser.mjs` usa Chromium real com uma instância **local
isolada**, professor fictício e banco descartável. Exige
`ASCEND_HTTP_BROWSER_BASE=http://127.0.0.1:PORTA/ascend` e recusa executar se o
pacote já existir. A validação passa pela interface normal: importação JSON,
publicação, visão aluno, seis workspaces, exploração HTTP, logs, submissões
incorretas, Visual Test Debugger, edição Monaco, submissões corretas e seis
itens concluídos. Capturas ficam em `/tmp/ascend-http-browser` (ou no diretório
`ASCEND_HTTP_BROWSER_OUTPUT`). Use um timeout externo de 300 segundos.

A pipeline `validate.yml` executa essa jornada com PostgreSQL, Redis e Judge
reais e sandbox Podman rootless. Os testes de API verificam permissões, testes
ocultos, isolamento de progresso e cadastro transacional. O teste administrativo
verifica reutilização sem duplicação e interrupção após uma tentativa ambígua.

Resultados locais não comprovam cadastro na instância de destino. A URL e a
contagem de produção devem vir da execução administrativa autenticada, depois
do deploy pela pipeline e da verificação pública. Sem esse acesso, a entrega
continua bloqueada nessa etapa e não deve ser anunciada como lista publicada
em produção.
