#!/usr/bin/env python3
"""Generate a reviewable, atomic statement-only SQL plan; never connect to a DB.

The operator must explicitly validate the plan before applying it with psql.
No publication, imports, test changes or submission writes are performed.
"""
import argparse
import json
import os
from pathlib import Path
from uuid import UUID

ROOT = Path(__file__).resolve().parents[1]
TEMPLATES = ROOT / 'api/internal/handler/activity_templates'


def literal(value):
    return "'" + value.replace("'", "''") + "'"


def plan(list_id, owner_id):
    list_id, owner_id = str(UUID(list_id)), str(UUID(owner_id))
    old = json.loads((TEMPLATES / 'fastapi-v1.json').read_text())
    new = json.loads((TEMPLATES / 'fastapi-pedagogy-v2.json').read_text())
    if len(old['items']) != 6 or len(new['items']) != 6:
        raise ValueError('Expected exactly six activities')
    for before, after in zip(old['items'], new['items']):
        if {k: v for k, v in before.items() if k != 'body'} != {k: v for k, v in after.items() if k != 'body'}:
            raise ValueError('Only statement changes are permitted')
    payload = [{'old': before, 'new_body': after['body']}
               for before, after in zip(old['items'], new['items'])]
    return f"""-- Review privately: contains the original assessment configuration.
-- Scope: list {list_id}; owner {owner_id}. No editorial changes.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
SET LOCAL standard_conforming_strings = on;
-- Prevent concurrent configuration edits during verification and update.
LOCK TABLE problem_lists, list_items, challenges, test_cases IN SHARE ROW EXCLUSIVE MODE;
DO $pedagogy$
DECLARE entry jsonb; item list_items%ROWTYPE; lab challenges%ROWTYPE;
BEGIN
 IF NOT EXISTS (SELECT 1 FROM problem_lists WHERE id='{list_id}' AND teacher_id='{owner_id}')
 OR (SELECT count(*) FROM list_items WHERE list_id='{list_id}') <> 6 THEN
   RAISE EXCEPTION 'List identity, owner or size differs';
 END IF;
 FOR entry IN SELECT value FROM jsonb_array_elements({literal(json.dumps(payload, ensure_ascii=False))}::jsonb) LOOP
   SELECT * INTO STRICT item FROM list_items WHERE list_id='{list_id}' AND title=entry->'old'->>'title';
   SELECT * INTO STRICT lab FROM challenges WHERE id=item.linked_challenge_id;
   IF item.body NOT IN (entry->'old'->>'body', entry->>'new_body')
   OR lab.description <> item.body OR item.difficulty::text <> entry->'old'->>'difficulty'
   OR lab.title <> item.title OR lab.difficulty::text <> item.difficulty::text
   OR lab.language <> 'fastapi' OR lab.lab_list_id IS DISTINCT FROM '{list_id}'::uuid
   OR lab.lab_owner_id IS DISTINCT FROM '{owner_id}'::uuid
   OR lab.starter_code IS DISTINCT FROM entry->'old'->'http_lab'->>'starter_code'
   OR lab.time_limit_ms IS DISTINCT FROM (entry->'old'->'http_lab'->>'time_limit_ms')::int
   OR lab.memory_limit_mb IS DISTINCT FROM (entry->'old'->'http_lab'->>'memory_limit_mb')::int
   OR (SELECT jsonb_agg(jsonb_build_object('input',input,'expected_output',expected_output,'is_sample',is_sample,'order_matters',order_matters) ORDER BY ordinal)
       FROM test_cases WHERE challenge_id=lab.id) IS DISTINCT FROM entry->'old'->'http_lab'->'test_cases' THEN
     RAISE EXCEPTION 'Activity diverged; transaction cancelled';
   END IF;
   UPDATE list_items SET body=entry->>'new_body' WHERE id=item.id AND body IS DISTINCT FROM entry->>'new_body';
   UPDATE challenges SET description=entry->>'new_body' WHERE id=lab.id AND description IS DISTINCT FROM entry->>'new_body';
 END LOOP;
END $pedagogy$;
COMMIT;
"""


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--list-id', required=True)
    parser.add_argument('--owner-id', required=True)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    # Exclusive creation prevents accidentally replacing an already reviewed plan.
    with os.fdopen(os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w') as output:
        output.write(plan(args.list_id, args.owner_id))
