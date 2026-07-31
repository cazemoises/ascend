import type { BeforeMount } from '@monaco-editor/react'

// Monaco's theme API wants hex strings (no '#' for token rule foregrounds,
// '#RRGGBB' for editor colors) — getComputedStyle resolves our oklch()
// tokens to rgb() strings, so this converts rather than hand-duplicating
// hex copies of --code-*/--accent-* that could drift from the real values.
function rgbToHex(rgbString: string): string {
  const channels = rgbString.match(/\d+(\.\d+)?/g)
  if (!channels) return '000000'
  const [r, g, b] = channels.map(Number)
  return [r, g, b].map((n) => Math.round(n).toString(16).padStart(2, '0')).join('')
}

function readToken(name: string): string {
  return rgbToHex(getComputedStyle(document.documentElement).getPropertyValue(name))
}

// Monaco's built-in tokenizers expose one generic "keyword" category per
// language (not a def/return-vs-for/in split like the source design's
// illustrative mockup) — this maps what Monaco actually gives us: keywords
// in the primary brand color, types in the secondary one, on the
// always-dark code surface (--code-* tokens, theme-invariant, so this
// never needs to redefine itself when the app theme toggles).
export const defineAscendMonacoTheme: BeforeMount = (monaco) => {
  const codeBg = readToken('--code-bg')
  const codeInk = readToken('--code-ink')
  const codeMuted = readToken('--code-muted')
  const terracota = readToken('--accent-terracota')
  const teal = readToken('--accent-teal')
  const amber = readToken('--accent-amber')

  monaco.editor.defineTheme('ascend', {
    base: 'vs-dark',
    inherit: true,
    rules: [
      { token: 'keyword', foreground: terracota },
      { token: 'type', foreground: teal },
      { token: 'type.identifier', foreground: teal },
      { token: 'number', foreground: amber },
      { token: 'comment', foreground: codeMuted, fontStyle: 'italic' },
    ],
    colors: {
      'editor.background': `#${codeBg}`,
      'editor.foreground': `#${codeInk}`,
      'editorLineNumber.foreground': `#${codeMuted}`,
      'editorLineNumber.activeForeground': `#${codeInk}`,
    },
  })
}
