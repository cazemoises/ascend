## Why

The challenge submission page uses a plain `<textarea>` for code input, which provides no syntax highlighting, indentation, or language-aware editing. Replacing it with Monaco Editor eliminates friction for users writing real code and aligns the editing experience with professional judge platforms.

## What Changes

- Install `@monaco-editor/react` as a frontend dependency
- Remove the `<textarea id="source_code">` from `ChallengePage.tsx`
- Render a `<Editor>` component from `@monaco-editor/react` in its place
- Wire the editor's `onChange` callback to the existing `sourceCode` state (same state used for submission)
- Map the language select value to the Monaco language identifier (python → `python`, go → `go`, javascript → `javascript`)
- Set theme to `vs-dark`, height to `400px`, font family to monospace

## Capabilities

### New Capabilities

- `code-editor`: Monaco-based code editor widget on the challenge page — syntax highlighting, language switching, dark theme, controlled value synced to submission state

### Modified Capabilities

- `web-frontend`: The challenge submission form now uses Monaco Editor instead of a textarea; production build must bundle Monaco workers correctly

## Impact

- **File**: `web/src/pages/ChallengePage.tsx` — editor replaces textarea
- **Dependency**: `@monaco-editor/react` added to `web/package.json`
- **Build**: Monaco ships web workers; Vite handles them automatically with `@monaco-editor/react` (no additional Vite config needed for dev; production build bundles them)
- **No API or backend changes**
