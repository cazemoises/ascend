-- One-off data seed script -- NOT a numbered schema migration. Run manually
-- (e.g. `psql -f scripts/fix_trilha_descriptions_multilang.sql`) against a
-- database that already has scripts/seed_trilha_multilang.sql and
-- scripts/seed_trilha_java.sql applied. Idempotent: safe to re-run, every
-- statement is a plain UPDATE keyed by slug.
--
-- Fixes 4 trilha-* challenge descriptions that were written before the
-- 41 challenges went multi-language (python/javascript/go/java) and still
-- named a Python-specific function/constant/builtin as if it were the only
-- language in play (e.g. "Use math.pi", ".swapcase()", "ord()", "max()").
-- Descriptions rewritten by the challenge author (Caze) to describe the
-- concept generically, or list each language's equivalent in parentheses
-- (backtick-quoted as inline code), rather than pointing at one language's
-- syntax.
--
-- This is a small, hand-picked set of 4 confirmed cases, not the output of
-- an automated scan across all 41 -- see chat transcript for context.

BEGIN;

-- trilha-n1-area-circulo
UPDATE challenges SET description = $DESC_1$Leia o raio `r`. Imprima a área do círculo (π × r²) com 2 casas decimais. Use a constante de PI da sua linguagem (`math.pi` em Python, `Math.PI` em Java/JavaScript, `math.Pi` em Go).$DESC_1$
WHERE slug = 'trilha-n1-area-circulo';

-- trilha-n1-inverte-caixa
UPDATE challenges SET description = $DESC_2$Leia uma linha de texto. Para cada letra, inverta maiúscula↔minúscula (números e símbolos ficam iguais). Não use funções prontas de inversão de caixa da sua linguagem (como `.swapcase()` em Python) — faça o loop manualmente.$DESC_2$
WHERE slug = 'trilha-n1-inverte-caixa';

-- trilha-n2-maior-sem-max
UPDATE challenges SET description = $DESC_3$Leia uma lista de inteiros (um por linha até o fim da entrada). Imprima o maior valor. NÃO use a função pronta de valor máximo da sua linguagem (como `max()` em Python/JS ou `Math.max` em Java) — percorra e compare manualmente.$DESC_3$
WHERE slug = 'trilha-n2-maior-sem-max';

-- trilha-n6-hash-funcao
UPDATE challenges SET description = $DESC_4$Leia uma chave (string) e um tamanho N. Calcule o hash: some os códigos ASCII/Unicode de cada caractere da chave (`ord()` em Python, `charCodeAt` em JS, cast pra int em Java/Go), e tire o resto da divisão por N. Imprima o resultado. **Primeiro passo pra entender tabela hash — pesquise o termo antes de começar.**$DESC_4$
WHERE slug = 'trilha-n6-hash-funcao';

COMMIT;
