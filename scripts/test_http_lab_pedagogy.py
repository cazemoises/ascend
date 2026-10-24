import json
import unittest
from plan_http_lab_pedagogy import plan, TEMPLATES


class PedagogyTest(unittest.TestCase):
    def test_only_narrative_changes_and_contract_preserved(self):
        old = json.loads((TEMPLATES / 'fastapi-v1.json').read_text())
        new = json.loads((TEMPLATES / 'fastapi-pedagogy-v2.json').read_text())
        self.assertEqual(len(new['items']), 6)
        for before, after in zip(old['items'], new['items']):
            contract = before['body'].split('## Contrato\n')[1].split('\n\nImplemente')[0]
            self.assertIn(contract, after['body'])
            for section in ['Contexto real', 'O problema', 'Sua missão', 'Exemplo de funcionamento', 'Por onde começar', 'Como saber que concluiu']:
                self.assertIn('## ' + section, after['body'])
            before.pop('body')
            after.pop('body')
            self.assertEqual(before, after)

    def test_plan_is_scoped_atomic_and_statement_only(self):
        sql = plan('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002')
        self.assertIn('BEGIN;', sql)
        self.assertTrue(sql.endswith('COMMIT;\n'))
        self.assertIn('IS DISTINCT FROM', sql)
        writes = [line.strip() for line in sql.splitlines() if line.strip().startswith('UPDATE ')]
        self.assertEqual(len(writes), 2)
        self.assertTrue(writes[0].startswith('UPDATE list_items SET body='))
        self.assertTrue(writes[1].startswith('UPDATE challenges SET description='))
        with self.assertRaises(ValueError):
            plan("'; DROP TABLE users;", 'invalid')
