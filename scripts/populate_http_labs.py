#!/usr/bin/env python3
"""Controlled HTTP activity import using the official API; never retries writes.

Authentication comes from an operator-provided Netscape cookie jar or an
already authenticated local forwarding service. Never inject identity headers
into a production deployment. Keep the checkpoint across invocations.
"""
import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import urllib.error
import urllib.parse
import urllib.request


def populate(base, package, checkpoint, request):
    me = request('GET', '/auth/me')
    if me.get('effective_role') != 'teacher':
        raise RuntimeError('An authenticated teacher session is required')
    inventory = request('GET', '/lists')
    matches = [row for row in inventory
               if row['title'].casefold() == package['title'].casefold()]
    if len(matches) > 1:
        raise RuntimeError('Multiple lists match; manual reconciliation required')
    titles = {item['title'].casefold() for item in package.get('items', [])}
    statements = {item['body'] for item in package.get('items', [])}
    for row in inventory:
        if matches and row['id'] == matches[0]['id']:
            continue
        existing = request('GET', '/lists/' + row['id'])
        if any(item['title'].casefold() in titles or item['body'] in statements
               for item in existing['items']):
            raise RuntimeError('Activities already exist in another visible list; no duplicates created')
    if matches:
        row = matches[0]
        if row['teacher_id'] != me['id']:
            raise RuntimeError('Matching list belongs to another teacher; no changes made')
        detail = request('GET', '/lists/' + row['id'])
    else:
        if checkpoint.exists():
            raise RuntimeError('Prior import attempt recorded; reconcile before another import')
        # Durable marker BEFORE sending the write. A lost response must never
        # cause this tool to create a second list on a subsequent invocation.
        with checkpoint.open('x') as output:
            json.dump({'base': base, 'title': package['title'], 'attempted': True}, output)
            output.flush()
            os.fsync(output.fileno())
        detail = request('POST', '/lists/import', package)
    if detail['teacher_id'] != me['id'] or len(detail['items']) != len(package['items']):
        raise RuntimeError('Persisted list does not match the package; publication stopped')
    for item, expected in zip(detail['items'], package['items']):
        for key in ('title', 'body', 'difficulty'):
            if item[key] != expected[key]:
                raise RuntimeError('Existing activity differs from package; no overwrite performed')
        challenge_id = item.get('linked_challenge_id')
        if not challenge_id:
            raise RuntimeError('Missing persisted Judge challenge')
        lab = request('GET', '/challenges/' + challenge_id)
        config = expected['http_lab']
        if any(lab[key] != expected[field] for key, field in
               (('title', 'title'), ('description', 'body'), ('difficulty', 'difficulty'))):
            raise RuntimeError('Persisted Judge statement differs')
        if lab['language'] != 'fastapi' or any(lab[key] != config[key] for key in
                ('starter_code', 'time_limit_ms', 'memory_limit_mb')):
            raise RuntimeError('Persisted laboratory configuration differs')
        tests = request('GET', '/challenges/' + challenge_id + '/test-cases')
        keys = ('input', 'expected_output', 'is_sample')
        if [{key: test[key] for key in keys} for test in tests] != [
                {key: test[key] for key in keys} for test in config['test_cases']]:
            raise RuntimeError('Persisted tests differ; publication stopped')
    if not detail['published']:
        request('PATCH', '/lists/' + detail['id'], {
            key: (True if key == 'published' else
                  detail[key][:10] if key in ('week_start', 'week_end') and detail.get(key)
                  else detail.get(key))
            for key in ('title', 'description', 'week_label', 'week_start', 'week_end', 'published')})
    # Verify publicly, without authentication, in the CLI caller as well.
    return {'id': detail['id'], 'title': detail['title'], 'activities': len(detail['items'])}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-url', required=True, help='Ascend root, including /ascend when applicable')
    parser.add_argument('--cookies', type=Path, help='Operator-provided Netscape cookie jar; never printed')
    parser.add_argument('--checkpoint', required=True, type=Path)
    parser.add_argument('--package', type=Path, default=Path(__file__).resolve().parents[1] /
                        'api/internal/handler/activity_templates/fastapi-v1.json')
    args = parser.parse_args()
    base = args.base_url.rstrip('/')
    url = urllib.parse.urlparse(base)
    if url.scheme != 'https' and not (url.scheme == 'http' and url.hostname in ('localhost', '127.0.0.1')):
        parser.error('HTTPS required except for local disposable validation')
    # Keep the deployment prefix, including cookie scope (/ascend in production).
    api = base + '/api/v1'
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            return None
    jar = http.cookiejar.MozillaCookieJar()
    if args.cookies:
        jar.load(str(args.cookies), ignore_discard=True)
    opener = urllib.request.build_opener(NoRedirect(), urllib.request.HTTPCookieProcessor(jar))

    def request(method, path, body=None):
        data = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(api + path, data=data, method=method,
                                     headers={'Content-Type': 'application/json'})
        with opener.open(req, timeout=40) as response:
            return json.load(response)

    lock = Path(str(args.checkpoint) + '.lock')
    descriptor = os.open(lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    try:
        package = json.loads(args.package.read_text())
        result = populate(base, package, args.checkpoint, request)
        with urllib.request.urlopen(api + '/lists/' + result['id'], timeout=15) as response:
            public = json.load(response)
        if not public['published'] or len(public['items']) != result['activities']:
            raise RuntimeError('Public visibility verification failed')
        result['url'] = base + '/listas/' + result['id']
        print(json.dumps(result, ensure_ascii=False))
    finally:
        os.close(descriptor)
        lock.unlink()


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, OSError, ValueError, urllib.error.URLError) as error:
        # Never print response bodies, credentials, or request headers.
        raise SystemExit('Population stopped: ' + (str(error) if isinstance(error, RuntimeError)
                                                  else type(error).__name__))
