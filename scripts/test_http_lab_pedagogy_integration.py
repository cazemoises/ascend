"""Run only against the explicitly named disposable rootless Podman DB."""
import json
import os
import subprocess
import unittest
from plan_http_lab_pedagogy import plan, literal, TEMPLATES

CONTAINER = os.environ.get('ASCEND_PEDAGOGY_TEST_DB')


@unittest.skipUnless(CONTAINER, 'Requires disposable Podman DB and browser fixtures')
class AdministrativeUpdateTest(unittest.TestCase):
    def sql(self, sql, succeeds=True):
        result = subprocess.run(['podman', 'exec', '-i', CONTAINER, 'psql', '-X', '-At',
                                 '-U', 'ascend_test', '-d', 'ascend_test', '-v', 'ON_ERROR_STOP=1'],
                                input=sql, text=True, capture_output=True, timeout=35)
        self.assertEqual(result.returncode == 0, succeeds, 'Unexpected SQL outcome (private output suppressed)')
        return result.stdout.strip()

    def snapshot(self):
        return self.sql("""SELECT jsonb_build_object(
          'lists',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM problem_lists t),
          'items',(SELECT jsonb_agg(to_jsonb(t)-'body' ORDER BY id) FROM list_items t),
          'labs',(SELECT jsonb_agg(to_jsonb(t)-'description' ORDER BY id) FROM challenges t),
          'tests',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM test_cases t),
          'submissions',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM submissions t));""")

    def test_atomic_idempotent_and_preserves_history(self):
        original = json.loads((TEMPLATES / 'fastapi-v1.json').read_text())
        identity = self.sql('SELECT id,teacher_id FROM problem_lists WHERE title=' + literal(original['title']) + ';').split('|')
        self.assertEqual(len(identity), 2, 'Run browser journey first on a fresh disposable DB')
        list_id, owner = identity
        scope = literal(list_id) + '::uuid'
        # Downgrade ONLY local fixture statements to exercise the real v1 -> v2 update.
        for item in original['items']:
            self.sql('UPDATE list_items SET body=' + literal(item['body']) + ' WHERE list_id=' + scope + ' AND title=' + literal(item['title']) + ';')
        self.sql('UPDATE challenges c SET description=i.body FROM list_items i WHERE i.list_id=' + scope + ' AND c.id=i.linked_challenge_id;')
        baseline = self.snapshot()
        update = plan(list_id, owner)
        self.sql(update)
        self.assertEqual(baseline, self.snapshot())
        bodies = self.sql('SELECT jsonb_agg(body ORDER BY title) FROM list_items WHERE list_id=' + scope + ';')
        revised = json.loads((TEMPLATES / 'fastapi-pedagogy-v2.json').read_text())
        self.assertEqual(json.loads(bodies), [item['body'] for item in revised['items']])
        self.assertEqual(self.sql('SELECT count(*) FROM list_items i JOIN challenges c ON c.id=i.linked_challenge_id WHERE i.list_id=' + scope + ' AND i.body IS DISTINCT FROM c.description;'), '0')
        self.sql(update)
        self.assertEqual(baseline, self.snapshot())
        self.assertEqual(bodies, self.sql('SELECT jsonb_agg(body ORDER BY title) FROM list_items WHERE list_id=' + scope + ';'))
        # A late divergent item must roll back any earlier updates in the loop.
        self.sql('UPDATE list_items SET body=body || chr(10) WHERE list_id=' + scope + ' AND title=' + literal(original['items'][-1]['title']) + ';')
        self.sql('UPDATE list_items SET body=' + literal(original['items'][0]['body']) + ' WHERE list_id=' + scope + ' AND title=' + literal(original['items'][0]['title']) + ';')
        self.sql('UPDATE challenges c SET description=i.body FROM list_items i WHERE i.list_id=' + scope + ' AND c.id=i.linked_challenge_id;')
        divergent = self.sql('SELECT jsonb_agg(body ORDER BY title) FROM list_items WHERE list_id=' + scope + ';')
        self.sql(update, succeeds=False)
        self.assertEqual(divergent, self.sql('SELECT jsonb_agg(body ORDER BY title) FROM list_items WHERE list_id=' + scope + ';'))
        self.assertEqual(baseline, self.snapshot())
        for item in revised['items']:
            self.sql('UPDATE list_items SET body=' + literal(item['body']) + ' WHERE list_id=' + scope + ' AND title=' + literal(item['title']) + ';')
        self.sql('UPDATE challenges c SET description=i.body FROM list_items i WHERE i.list_id=' + scope + ' AND c.id=i.linked_challenge_id;')
