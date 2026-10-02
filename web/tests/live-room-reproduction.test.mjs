import assert from 'node:assert/strict'
import test from 'node:test'
import * as Y from 'yjs'

test('reproduces shared-document text movement while both students type', () => {
  const students = [new Y.Doc(), new Y.Doc()]
  const pending = []
  students.forEach((doc, index) => {
    doc.clientID = index + 1
    doc.on('update', (update, origin) => {
      if (origin !== 'remote') pending.push({ index, update })
    })
  })
  const expected = ['print(1)\n', 'print(2)\n']
  const positions = [0, 0]
  for (let i = 0; i < expected[0].length; i++) {
    students.forEach((doc, index) => {
      doc.getText('code').insert(positions[index]++, expected[index][i])
    })
    for (const { index, update } of pending.splice(0)) {
      // The editor caret is local, but receiving another student's insert
      // still changes the text at its offset in the shared document.
      Y.applyUpdate(students[1 - index], update, 'remote')
    }
  }
  const texts = students.map(doc => doc.getText('code').toString())
  assert.equal(texts[0], texts[1])
  assert.notEqual(texts[0], expected[0])
  assert.notEqual(texts[1], expected[1])
  console.log('Shared output:', JSON.stringify(texts[0]))
  students.forEach(doc => doc.destroy())
})

test('late participant hydrates omitted snapshot edits from the durable journal', () => {
  const active = new Y.Doc()
  const journal = []
  active.on('update', update => journal.push(update))
  active.getText('code').insert(0, 'first\n')
  const snapshot = Y.encodeStateAsUpdate(active)
  active.getText('code').insert(6, 'missing\n')
  const late = new Y.Doc()
  Y.applyUpdate(late, snapshot)
  // The server retains every committed update, independently of snapshots.
  for (const update of journal) Y.applyUpdate(late, update)
  let nextUpdate
  active.on('update', update => { nextUpdate = update })
  active.getText('code').insert(active.getText('code').length, 'next\n')
  Y.applyUpdate(late, nextUpdate)
  assert.equal(late.getText('code').toString(), active.getText('code').toString())
})
