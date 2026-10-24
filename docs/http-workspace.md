# Workspace de resolução dos Laboratórios HTTP

Os laboratórios existentes abrem pela mesma rota `/ascend/challenges/:id` e pela mesma `ChallengePage` dos desafios tradicionais. A listagem e a situação editorial das atividades não são alteradas.

## Composição compartilhada

- O enunciado usa o mesmo `ReactMarkdown`, `rehypeHighlight`, `.markdown-content`, cabeçalho, dificuldade, `TelemetryChip` e exemplos do workspace tradicional.
- `CodeEditor` extrai a configuração anterior do Monaco: tema `ascend`, fonte/tamanho, snippets, autocomplete, minimap desativado e atalhos nativos. FastAPI usa o modo Python; os desafios tradicionais também usam esse componente.
- A mesma estrutura `.workspace`, `.workspace__brief`, `.workspace__editor`, toolbar e barra de ações controla ambos os modos. O breakpoint continua em 1020px e o editor mobile mantém 420px.
- `SubmissionResult` extrai a apresentação anterior de `SubmissionPage`: `ExecutionTrail`, `VerdictBadge`, `TelemetryChip`, comparação e logs. O painel HTTP o reutiliza junto ao editor; o resultado salvo continua usando o mesmo componente.
- `HTTPTestDebugger` e `httpDebuggerView` continuam decidindo a visibilidade dos dados. A visualização formatada de respostas recebe somente dados previamente autorizados; testes ocultos e respostas contaminadas por seu estado continuam protegidos.

Foram removidos `HTTPLabWorkspace`, seu Monaco independente, o layout `.lab-workspace` e a textarea do array de requisições. Os envelopes do Judge nos exemplos antigos são apresentados como exemplos HTTP, sem modificar o Markdown persistido, os testes ou os templates cadastrados.

## Uso na interface

1. Em **Listas**, abra a lista existente de Laboratórios HTTP e escolha **resolver desafio**.
2. Leia o enunciado e edite `main.py`. Os atalhos do Monaco, incluindo autocomplete (`Ctrl+Space`), busca e desfazer/refazer, são os mesmos dos demais desafios.
3. Em **Cliente HTTP**, escolha uma requisição do lote, método, path, query e corpo JSON opcional. Adicione/remova requisições para montar uma sequência; elas executam em ordem no mesmo servidor. Cada envio inicia um ambiente novo.
4. Use **executar servidor e testar rotas** para explorar sem nota. **Resultados** mostra status HTTP e corpo formatado de cada resposta; expanda os headers. A visão técnica inclui snapshots e logs autorizados.
5. Use **submeter para avaliação** (ou **testar avaliação como mentor**) na barra compartilhada. Um erro público abre a comparação estruturada no Visual Test Debugger. Selecione divergências e alterne entre visão simples/técnica. **Abrir resultado salvo** mantém a navegação anterior e permite retornar ao código enviado.

O contrato existente aceita método, path/query e JSON; define `Content-Type: application/json` quando há corpo. A interface explicita esse header automático. Headers de requisição arbitrários não são suportados pelo Judge atual; não são silenciosamente enviados ou simulados. Headers de resposta continuam limitados à seleção pedagógica já implementada no sandbox.

## Validação reproduzível

A pipeline `Validate Ascend` executa `web/tests/http-labs-browser.mjs` em Chromium contra PostgreSQL/Redis/Judge descartáveis, em Podman rootless. O script recusa hosts externos. Seu import/publicação de fixtures ocorre somente nesse ambiente local, nunca nas listas reais.

O cenário compara um desafio tradicional e um laboratório nas mesmas resoluções (1440×1000 e 390×844), exercita snippets, edição, desfazer/refazer, submissão tradicional e retorno, exploração dos seis laboratórios, erros públicos, seleção de divergências, modos simples/técnico, debugger mobile, correção aceita, resultado salvo, restauração de código e progresso. Também verifica edição visual de método/query/body, erro de JSON antes do envio e lote com duas requisições. Capturas e medidas ficam no artefato `http-labs-browser` da pipeline.

Testes unitários cobrem serialização ordenada, query repetida, JSON nulo, entrada inválida, adaptação dos exemplos e proteção dos dados ocultos na apresentação formatada. Todos os comandos de validação devem ter timeout, conforme a pipeline.

## Auditoria local — 24/10/2026

A comparação foi executada no aplicativo real, com API, Redis, PostgreSQL e Judge descartáveis, sem interceptar respostas HTTP. As capturas desktop e mobile foram inspecionadas visualmente, incluindo a comparação esperado/recebido no debugger. Foram corrigidos o corte do botão de avaliação com rótulos longos, o envio de exploração antes escondido pela rolagem do formulário e os nomes acessíveis dos seletores/body após edição. Os dois modos mantêm as mesmas larguras de painéis e cores; no mobile, ambos usam o editor de 420px sem overflow horizontal da página.

Passaram os cenários dos seis laboratórios com Judge real, navegação/recuperação do código, proteção de resultados ocultos e conclusão dos seis itens na lista descartável. Também passaram todos os pacotes Go, os 14 testes de frontend, lint e build. Essa evidência local não substitui os checks da pipeline no HEAD do PR nem afirma que houve deploy.
