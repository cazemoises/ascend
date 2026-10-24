import copy
from pathlib import Path
import tempfile
import unittest
from populate_http_labs import populate


class PopulationTests(unittest.TestCase):
    def test_resume_verifies_existing_and_never_reimports(self):
        package = {'title': 'HTTP', 'items': [{'title': 'One', 'body': 'Statement',
            'difficulty': 'easy', 'http_lab': {'starter_code': 'code', 'time_limit_ms': 1000,
            'memory_limit_mb': 128, 'test_cases': [{'input': '{}', 'expected_output': '{}', 'is_sample': True}]}}]}
        detail = {'id': 'list', 'teacher_id': 'mentor', 'title': 'HTTP', 'published': False,
                  'week_start': '2026-10-19T00:00:00Z', 'week_end': None,
                  'items': [{**package['items'][0], 'linked_challenge_id': 'lab'}]}
        calls = []
        def request(method, path, body=None):
            calls.append((method, path))
            if path == '/auth/me': return {'id': 'mentor', 'effective_role': 'teacher'}
            if path == '/lists': return [detail]
            if path == '/lists/list':
                if method == 'PATCH':
                    self.assertEqual(body['week_start'], '2026-10-19')
                    detail['published'] = body['published']
                return detail
            if path == '/challenges/lab': return {'language': 'fastapi', 'title': 'One', 'description': 'Statement', 'difficulty': 'easy', **package['items'][0]['http_lab']}
            if path.endswith('/test-cases'): return package['items'][0]['http_lab']['test_cases']
            self.fail('Unexpected API call')
        with tempfile.TemporaryDirectory() as directory:
            checkpoint = Path(directory) / 'checkpoint'
            checkpoint.write_text('prior attempt')
            for _ in range(2):
                result = populate('https://example.test/ascend', package, checkpoint, request)
                self.assertEqual(result['activities'], 1)
            self.assertFalse(any(method == 'POST' for method, _ in calls))
            self.assertEqual(sum(method == 'PATCH' for method, _ in calls), 1)
            changed = copy.deepcopy(package)
            changed['items'][0]['body'] = 'different'
            with self.assertRaisesRegex(RuntimeError, 'differs'):
                populate('https://example.test', changed, checkpoint, request)

    def test_lost_import_response_records_attempt_before_write(self):
        writes = []
        with tempfile.TemporaryDirectory() as directory:
            checkpoint = Path(directory) / 'checkpoint'
            def request(method, path, body=None):
                if path == '/auth/me': return {'id': 'mentor', 'effective_role': 'teacher'}
                if path == '/lists': return []
                self.assertTrue(checkpoint.exists())
                writes.append(path)
                raise TimeoutError('Response lost')
            with self.assertRaises(TimeoutError):
                populate('https://example.test', {'title': 'HTTP'}, checkpoint, request)
            with self.assertRaisesRegex(RuntimeError, 'Prior import attempt'):
                populate('https://example.test', {'title': 'HTTP'}, checkpoint, request)
            self.assertEqual(writes, ['/lists/import'])

    def test_ambiguous_attempt_without_visible_list_stops(self):
        def request(method, path, body=None):
            self.assertEqual(method, 'GET')
            return {'id': 'mentor', 'effective_role': 'teacher'} if path == '/auth/me' else []
        with tempfile.TemporaryDirectory() as directory:
            checkpoint = Path(directory) / 'checkpoint'
            checkpoint.write_text('attempted')
            with self.assertRaisesRegex(RuntimeError, 'Prior import attempt'):
                populate('https://example.test', {'title': 'HTTP'}, checkpoint, request)


if __name__ == '__main__':
    unittest.main()
