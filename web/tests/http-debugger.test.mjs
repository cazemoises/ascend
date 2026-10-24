import assert from 'node:assert/strict'
import test from 'node:test'
import { readFile } from 'node:fs/promises'
import ts from 'typescript'
import { renderToStaticMarkup } from 'react-dom/server'
import { createElement } from 'react'
import { httpDebuggerView } from '../src/lib/httpDebugger.ts'

// Compile only this component in memory; keep the app's real React renderer.
let compiled = ts.transpileModule(await readFile(new URL('../src/components/HTTPTestDebugger.tsx', import.meta.url), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2023 },
}).outputText
for (const name of ['react/jsx-runtime', 'react']) compiled = compiled.replaceAll(`from "${name}"`, `from ${JSON.stringify(import.meta.resolve(name))}`).replaceAll(`from '${name}'`, `from ${JSON.stringify(import.meta.resolve(name))}`)
compiled = compiled.replaceAll("'../lib/httpDebugger'", JSON.stringify(new URL('../src/lib/httpDebugger.ts', import.meta.url).href))
const { HTTPTestDebugger } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`)

const base = { id: 'fixture', language: 'fastapi', status: 'wrong_answer', failed_is_sample: true,
  stdout: '{"status":200,"json":"<script>alert(1)</script>"}', expected_output: '{"status":201,"json":null}', stderr: 'public logs',
  http_debug: { differences: [{path:'$.status',kind:'status',expected:'201',actual:'200',explanation:'O código HTTP recebido difere do esperado.'}],truncated:false} }

test('public snapshots show selectable deterministic differences side by side and escape HTML', () => {
  const html = renderToStaticMarkup(createElement(HTTPTestDebugger, {submission:base}))
  for (const text of ['Visão simples','Visão técnica','Esperado','Recebido','$.status','aria-pressed="true"','O código HTTP']) assert.ok(html.includes(text), text)
  assert.ok(!html.includes('<script>'))
  assert.ok(!html.includes('JSON bruto'))
  const malicious = {...base,http_debug:{differences:[{...base.http_debug.differences[0],actual:'"<script>alert(1)</script>"'}],truncated:false}}
  const escaped=renderToStaticMarkup(createElement(HTTPTestDebugger,{submission:malicious}))
  assert.ok(escaped.includes('&lt;script&gt;'))
  assert.ok(!escaped.includes('<script>'))
  assert.equal(httpDebuggerView(base).kind, 'comparison')
})
test('hidden and legacy responses fail closed even with malicious snapshot/debug fields', () => {
  for (const failed_is_sample of [false, null, undefined]) {
    const sub = {...base, failed_is_sample, stdout:'private actual',expected_output:'private answer',stderr:'private logs'}
    const html = renderToStaticMarkup(createElement(HTTPTestDebugger,{submission:sub}))
    assert.ok(!html.includes('private'))
    assert.ok(html.includes('indisponível'))
    assert.equal(httpDebuggerView(sub).expected,null)
  }
})
test('exploration has no expected answer and mentor test-run alone grants no visibility', () => {
  const view = httpDebuggerView({...base,http_exploration:true})
  assert.equal(view.kind,'exploration');assert.equal(view.expected,null);assert.equal(view.comparison,undefined)
  assert.equal(httpDebuggerView({...base,is_test_run:true,failed_is_sample:false}).kind,'unavailable')
  assert.equal(httpDebuggerView({...base,language:'python'}),null)
  assert.equal(httpDebuggerView({...base,status:'pending'}),null)
})
test('safe all-public execution keeps responses and logs without fabricating comparison', () => {
  const view=httpDebuggerView({...base,http_feedback_safe:true,http_debug:undefined,status:'runtime_error'})
  assert.equal(view.kind,'response');assert.equal(view.logs,'public logs');assert.equal(view.expected,null)
})
