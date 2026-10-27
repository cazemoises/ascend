# ascend

### Pool de conexões da API

A API limita o pool PostgreSQL a 20 conexões abertas e 5 ociosas por processo.
`DB_MAX_OPEN_CONNS` e `DB_MAX_IDLE_CONNS` aceitam inteiros positivos para ajustar
esses limites; conexões ociosas expiram após 5 minutos. Dimensione a soma dos
pools de todas as réplicas abaixo do limite do PostgreSQL, reservando conexões
para o judge, migrações e administração. Picos aguardam uma conexão livre.
Falhas na identificação registram `identity lookup failed`, com `request_id` e
`sqlstate`, sem email ou detalhes SQL. SQLSTATE `53300` indica excesso de conexões;
os logs são necessários para confirmar se essa foi a causa de um erro 500.

Contas provisionadas pelo Pangolin usam um marcador aleatório inválido como
hash de senha na coluna legada; não calculam bcrypt no primeiro acesso.
