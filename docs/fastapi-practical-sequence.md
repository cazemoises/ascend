# Preparação da avaliação prática FastAPI

## Acesso do mentor

No endereço habitual do portal, abra `/ascend/listas/nova/laboratorios`, ou use **listas → Preparar sequência FastAPI com IA**. É necessário estar autenticado pelo Pangolin como professor. A tela carrega o pacote do backend somente para mentores, com `Cache-Control: no-store`; os testes ocultos não entram no bundle do aluno.

A sequência **Laboratórios HTTP · Primeiros passos com FastAPI** contém seis atividades independentes:

1. GET e resposta JSON.
2. POST, corpo Pydantic e status 201.
3. Parâmetros de caminho e consulta.
4. Validação com limites e resposta 422 normalizada.
5. CRUD em memória, IDs sequenciais e 404.
6. Erros de domínio: 400, 404, 409 e preservação de estoque.

Os enunciados, códigos iniciais, exemplos e testes públicos/ocultos foram produzidos por IA durante a preparação. A correção usa exclusivamente o executor e o comparador determinístico existentes.

Revise os enunciados e abra **Revisar código inicial e testes públicos/ocultos**. O pacote também pode ser editado antes da aprovação ou substituído por outra sequência produzida por IA no mesmo formato. Após editar o JSON, use **Atualizar revisão**. Marque a aprovação e use **Aprovar e cadastrar sequência em rascunho**: a API oficial `POST /api/v1/lists/import` cria lista, itens, desafios e testes em uma única transação. Ela aceita o campo opcional `items[].http_lab` com o mesmo contrato de `POST /list-items/{id}/http-lab`. Pacotes inválidos não deixam cadastros parciais. O mentor é definido pela sessão, nunca pelo JSON; nenhuma lista existente é modificada. A lista é sempre criada em rascunho, mesmo que o pacote contenha `published`.

A tela alerta sobre listas próprias com o mesmo título. Se a conexão falhar durante o cadastro, confira **listas** antes de reabrir a importação: a confirmação pode ter se perdido após o commit. Não há repetição automática. As configurações HTTP são imutáveis depois da criação; uma nova versão exige novos itens.

Na página da lista criada, confira os itens e use **publicar** para torná-los acessíveis na experiência aluno. A publicação não é feita pelo carregamento ou pela importação do pacote.

## Primeiros passos na avaliação pessoal

Use **ver como aluno** na barra flutuante, volte à lista publicada e abra o exercício 01. Para retornar à autoria, use **ver como professor**. Rascunhos não aparecem para alunos. A avaliação de mentor é um teste e não conta no progresso; uma submissão aceita na visão aluno conclui o item.

1. Copie o lote JSON do enunciado para o campo de requisições e use **Explorar**, sem nota.
2. Envie o código inicial para avaliação: ele contém um erro proposital. Selecione uma diferença na **Visão simples** do Visual Test Debugger; abra a **Visão técnica** para conferir requisição e snapshots públicos.
3. Corrija o código e envie novamente até aceitar. Avance para POST e para os demais exercícios.

Cada avaliação reinicia a aplicação. Requisições do mesmo lote compartilham estado. Os testes públicos precedem os ocultos; pedidos, gabaritos e logs protegidos continuam sujeitos às regras existentes. Falhas ocultas não mostram comparação detalhada.

## Limite da preparação automatizada

A execução de preparação não possuía uma sessão Pangolin de mentor para consultar o inventário real ou cadastrar/publicar atividades na instância. Não foram injetados headers de identidade, acessados bancos de produção ou alteradas listas existentes. O pacote e o fluxo oficial de aprovação ficam disponíveis pelo deploy da mudança; o cadastro real ocorre quando o mentor o aprova na aplicação. O prefixo `/ascend/` vem da configuração atual do frontend; use o domínio habitual do portal.

## Validação

- Testes unitários Go de todos os módulos e frontend; lint e build do frontend.
- Catálogo restrito a professor, não cacheável; validação dos seis contratos e ordem público/oculto.
- PostgreSQL 16 descartável no Podman rootless: importação completa em rascunho, acesso do proprietário, invisibilidade pública e rollback de erro após cinco laboratórios.
- Sandbox FastAPI real no Podman rootless: cada solução de referência aceita em todos os casos e cada código inicial produz feedback público de erro.

Os dois testes de integração foram acrescentados ao workflow oficial de validação. Esta verificação prepara atividades utilizáveis; não representa uma avaliação de UX pelo proprietário.
