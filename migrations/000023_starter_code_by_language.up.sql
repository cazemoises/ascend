-- Structured per-language starter_code: maps a language key (python,
-- javascript, go, ...) to its own marker-delimited starter code, so a
-- multi-language challenge can carry a distinct stdin/stdout harness per
-- language instead of one text blob only ever safe to splice under the
-- language it was written for. NULL/absent keeps every existing challenge's
-- behavior unchanged (see challengeRecord.starterCodeFor in the judge
-- worker), including the legacy single-language starter_code column, which
-- stays in place as the fallback for challenges that never adopt this.
ALTER TABLE challenges ADD COLUMN IF NOT EXISTS starter_code_by_language JSONB NULL;
