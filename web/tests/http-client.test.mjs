import assert from 'node:assert/strict'
import test from 'node:test'
import { httpDescription, initialRequests, requestDraft, serializeRequests } from '../src/lib/httpClient.ts'

test('public examples round-trip stateful batches including query and null JSON', () => {
 const batch=[{method:'POST',path:'/items',json:{name:'Olá'}},{method:'GET',path:'/items?q=a+b&q=c'},{method:'PATCH',path:'/items/1',json:null}]
 const drafts=initialRequests(batch.map(request=>({input:JSON.stringify(request),expected_output:'{}'})))
 assert.deepEqual(serializeRequests(drafts),batch)
 assert.equal(drafts[1].path,'/items');assert.equal(drafts[1].query,'q=a+b&q=c')
 assert.equal(drafts[2].body,'null')
})
test('empty examples get an editable GET and malformed samples do not crash the workspace', () => {
 assert.deepEqual(serializeRequests(initialRequests([])),[{method:'GET',path:'/'}])
 assert.equal(initialRequests([{input:'bad'}])[0].path,'/')
})
test('invalid body, external paths and invalid batch sizes fail before sending', () => {
 for(const path of ['https://example.test','//example.test','/items?x=1','/a\\b','/a#hash']) {
  assert.throws(()=>serializeRequests([{method:'GET',path,query:'',body:''}]),/path relativo/)
 }
 assert.throws(()=>serializeRequests([{method:'POST',path:'/',query:'',body:'{'}]),/JSON inválido/)
 assert.throws(()=>serializeRequests([]),/1 a 10/)
 assert.throws(()=>serializeRequests(Array.from({length:11},()=>requestDraft({method:'GET',path:'/'}))),/1 a 10/)
})

test('authored request envelopes become HTTP examples while Markdown hierarchy and ordinary JSON remain', () => {
 const statement='## Objetivo\nCole este lote no campo de requisições e use **Explorar** (sem nota):\n\n```json\n[{"method":"POST","path":"/items","json":{"name":"Olá"}}]\n```\n\n## Contrato\n```json\n{"name":"exemplo"}\n```'
 const rendered=httpDescription(statement)
 assert.ok(rendered.includes('## Objetivo'));assert.ok(rendered.includes('## Contrato'))
 assert.ok(rendered.includes('POST /items'));assert.ok(rendered.includes('Content-Type: application/json'))
 assert.ok(!rendered.includes('"method"'));assert.ok(!rendered.includes('Cole este lote'))
 assert.ok(rendered.includes('```json\n{"name":"exemplo"}'))
})
