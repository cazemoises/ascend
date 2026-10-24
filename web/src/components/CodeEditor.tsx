import Editor from '@monaco-editor/react'
import { registerAscendSnippets } from '../lib/monacoSnippets'
import { defineAscendMonacoTheme } from '../lib/monacoTheme'

// One configuration for every challenge, including HTTP labs. Monaco owns
// editing, search, undo/redo and completion keyboard shortcuts in both modes.
export function CodeEditor({ language, value, onChange, readOnly }: {
  language: string; value: string; onChange: (value: string) => void; readOnly: boolean
}) {
  return <Editor height="100%" theme="ascend" language={language} value={value}
    beforeMount={(monaco) => {
      defineAscendMonacoTheme(monaco)
      registerAscendSnippets(monaco)
    }}
    onChange={(value) => onChange(value ?? '')}
    options={{
      fontFamily: 'JetBrains Mono, Consolas, monospace', fontSize: 13,
      minimap: { enabled: false }, automaticLayout: true, padding: { top: 12 }, readOnly,
      quickSuggestions: { other: true, comments: false, strings: false },
      wordBasedSuggestions: 'matchingDocuments', snippetSuggestions: 'inline',
    }} />
}
