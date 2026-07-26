## 1. Dependency

- [x] 1.1 Install `@monaco-editor/react` — run `npm install @monaco-editor/react` in `web/`
- [x] 1.2 Verify TypeScript types are included (the package ships its own types; no `@types/` needed)

## 2. Editor Component

- [x] 2.1 In `web/src/pages/ChallengePage.tsx`, add `import Editor from '@monaco-editor/react'` at the top
- [x] 2.2 Remove the `<textarea id="source_code" ...>` element
- [x] 2.3 Replace it with `<Editor height="400px" theme="vs-dark" language={language} value={sourceCode} onChange={(v) => setSourceCode(v ?? '')} options={{ fontFamily: 'monospace', readOnly: submitting }} />`
- [x] 2.4 Remove the now-unused `<label htmlFor="source_code">` or update its `htmlFor` attribute (Monaco doesn't use a native label association)

## 3. Verification

- [x] 3.1 Run `cd web && npx tsc --noEmit` — must pass with no type errors
- [x] 3.2 Run `cd web && npm run lint` — must pass with no lint errors
- [ ] 3.3 Start the dev server (`npm run dev`) and navigate to a challenge page — verify Monaco renders with dark theme and 400px height
- [ ] 3.4 Select "go" from the language dropdown — verify Go syntax highlighting activates
- [ ] 3.5 Type code and submit — verify the correct source code reaches the API (check network tab)
- [ ] 3.6 Click Submit while a request is in flight — verify the editor is in read-only mode
<!-- browser verification: run npm run dev and test manually -->

## 4. Commit

- [x] 4.1 `git add web/package.json web/package-lock.json web/src/pages/ChallengePage.tsx`
- [x] 4.2 `git commit -m "feat(web): replace textarea with Monaco Editor on challenge page"`
