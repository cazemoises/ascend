## Context

The challenge page (`ChallengePage.tsx`) currently uses a plain HTML `<textarea>` for code input. The textarea provides no syntax highlighting, auto-indentation, or language awareness. `@monaco-editor/react` is the standard React wrapper around the Monaco editor (same engine as VS Code) and ships with first-class Vite support via its built-in worker configuration.

## Goals / Non-Goals

**Goals:**
- Replace `<textarea>` with Monaco Editor (syntax highlighting, language modes, dark theme)
- Keep the editor value in sync with the existing `sourceCode` React state so the submission handler is unchanged
- Language mode updates when the user changes the language `<select>`
- Controlled install: one package, no custom Vite worker plugin needed

**Non-Goals:**
- Custom keybindings or editor extensions
- Code completion / IntelliSense (requires language server)
- Full-screen or resizable editor
- Persisting code across page reloads

## Decisions

### Use `@monaco-editor/react` over raw `monaco-editor`

`@monaco-editor/react` handles worker setup automatically (no manual `MonacoWebpackPlugin` or Vite worker config), lazy-loads Monaco via a dynamic import, and exposes a simple `<Editor>` component. The alternative — importing `monaco-editor` directly and configuring workers manually — adds ~30 lines of Vite boilerplate and is fragile across Monaco versions.

### Controlled component via `onChange`

The editor exposes `onChange?: (value: string | undefined) => void`. We pass `(v) => setSourceCode(v ?? '')` so the existing `sourceCode` state remains the single source of truth. The alternative (uncontrolled with a ref) would require reading `editor.getValue()` in the submit handler and is unnecessary complexity.

### Language ID mapping at the call site

Monaco uses `"python"`, `"go"`, `"javascript"` — identical to the `SubmissionLanguage` union already in `api.ts`. No mapping table needed; the `language` state value is passed directly as `language={language}` to `<Editor>`.

### Height fixed at `400px`, theme `vs-dark`

These are the user's explicit requirements. Height as a prop string (`"400px"`) is simpler than CSS; `vs-dark` is Monaco's built-in dark theme and requires no additional theme loading.

## Risks / Trade-offs

- **Bundle size**: Monaco adds ~2 MB to the production bundle (gzipped ~600 KB). For an internal/dev-stage tool this is acceptable. If bundle size becomes a concern, lazy-load the entire `ChallengePage` via `React.lazy`.
- **Initial load latency**: Monaco loads workers asynchronously. The editor mounts with a brief loading state handled by `@monaco-editor/react`'s built-in `loading` prop (defaults to "Loading…"). No custom spinner needed.
- **`value` vs `defaultValue`**: Using `value={sourceCode}` makes this a fully controlled editor. Every keystroke triggers `onChange` → `setSourceCode` → re-render. For a 400px editor this is imperceptible, but if performance issues arise, switch to `defaultValue` + ref.
