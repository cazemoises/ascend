# Visual Test Debugger dos Laboratórios HTTP

O debugger reutiliza o resultado salvo da submissão FastAPI no workspace e na página de resultado. Não executa código adicional, não usa IA e não muda os exercícios tradicionais.

## Uso e contrato

- A visão simples lista diferenças selecionáveis com caminho JSONPath, explicação determinística e valores esperado/recebido lado a lado. Ausência de um campo é diferente do valor JSON `null`.
- A visão técnica acrescenta a requisição pública, snapshots JSON brutos e logs permitidos. A exploração continua sem nota e sem resposta esperada.
- Status HTTP e JSON mantêm as regras existentes. Objetos ignoram ordem das chaves; arrays comparam índices e tamanhos; tipos diferentes falham; números são comparados com precisão racional, sem arredondamento de JavaScript.
- `expected_output` aceita `headers` opcional, por exemplo `{"status":200,"headers":{"Content-Type":"application/json"},"json":{"ok":true}}`. Headers declarados são obrigatórios, com nomes sem distinção de maiúsculas e valores exatos. Headers extras não afetam a nota, preservando Labs existentes sem expectativas de headers.
- São permitidos somente `content-type`, `content-language`, `cache-control`, `allow` e `content-encoding`. Cookies, autenticação, chaves e metadados de infraestrutura não são capturados. Nomes duplicados após normalização e valores com CR/LF são rejeitados.

A API acrescenta `http_debug` com `differences` e `truncated` somente quando os snapshots são públicos, válidos e correspondem ao caso que falhou. Cada diferença contém `path`, `kind`, `explanation`, `expected` e `actual`; os valores são literais JSON em strings, preservando números grandes. `null` no campo de diferença significa ausência; a string `"null"` representa o valor JSON null. `http_exploration` identifica a exploração, e `http_feedback_safe` identifica resultados e logs de execuções sem casos ocultos. Não há migração de banco.

## Visibilidade e limites

A avaliação continua parando no primeiro erro para determinar o feedback. O debugger não busca nem envia gabaritos ocultos. Não há ampliação de acesso: aluno lê somente suas submissões; permissões do mentor são as existentes.

Um lote HTTP compartilha estado. Portanto, somente casos públicos anteriores ao primeiro caso oculto podem fornecer snapshots; um caso público posterior pode carregar dados ocultos e é protegido. Os logs do lote inteiro são suprimidos se houver qualquer caso oculto, inclusive posterior à falha pública. O Judge aplica essa regra ao salvar e a API a verifica novamente ao ler resultados antigos. A posição da falha (`passed_count`), a requisição, a resposta esperada e a ordem dos casos precisam corresponder, impedindo que requisições duplicadas contornem a proteção. Resultados antigos incompletos ou truncados ficam sem comparação detalhada.

A comparação limita o diagnóstico a 100 diferenças e 32 níveis. O envelope de cada resposta continua limitado a 16 KiB, o lote a 30 requisições avaliadas ou 10 exploratórias, e a saída do sandbox a 64 KiB. Snapshots HTTP usam esse teto de armazenamento para evitar truncar JSON válido aos 8000 bytes dos exercícios tradicionais; os limites destes permanecem intactos. React renderiza caminhos, corpos e logs como texto, sem HTML executável.

## Validação realizada

Todos os comandos de testes usam `timeout -k 2s 58s`; testes Go usam também `-timeout 40s`, executados por pacote, sequencialmente. Logs locais: `/tmp/ascend-visual-debugger/`.

- Todos os pacotes Go: aprovados.
- Comparador: status, headers ausentes/incorretos, JSON aninhado, ausentes/extras, tipos, arrays, precisão numérica, caminhos com caracteres especiais, corpo não JSON, limites e headers proibidos.
- Judge em Podman rootless, com race detector: dez cenários aprovados, incluindo headers filtrados, divergência de headers, estado, isolamento, budgets, timeout e remoção do sandbox; testes adicionais de visibilidade em lotes mistos aprovados.
- PostgreSQL descartável em v29 + Redis + Judge reais, com race detector: criação pelo mentor, permissões, exploração sem progresso, feedback público para aluno/mentor, falha oculta sem detalhes, proteção de resultados antigos e requisições duplicadas, submissão aceita e execução do mentor sem nota.
- Regressões da API: dashboard, histórico, avaliações e submissões aprovados com race detector.
- Frontend: testes, lint e build aprovados. Chromium em container rootless: seleção, modos, snapshots/logs, mentor/aluno, caso oculto e layout de 390px aprovados; API simulada para esta verificação de interação, separada da integração real acima.

A publicação e o deploy devem ser confirmados nos checks do PR e nos workflows do SHA integrado; este documento não substitui essa evidência.
