import type { BeforeMount } from '@monaco-editor/react'

// getComputedStyle().getPropertyValue() on a *custom property* returns its
// raw specified text verbatim (e.g. "oklch(24% 0.02 280)") — custom
// properties are stored as unparsed token streams, not resolved computed
// colors, unlike real CSS properties such as `color`. A prior version of
// this file assumed it got back an rgb() string and regex-matched the
// numbers out of whatever text it actually got; fed an oklch() string, that
// misread [L%, C, H] as [r, g, b] — e.g. a 280deg hue over 255 overflowed a
// byte ("118" instead of "18"), producing malformed hex like "#1800118"
// that crashes Monaco's theme validator, while other tokens quietly
// resolved to valid-looking but *wrong* colors instead of crashing at all.
//
// The canvas 2D context's fillStyle setter, by contrast, actually parses
// any valid CSS color syntax (oklch included) the way a renderer would,
// and reading the pixel back after fillRect gives real 0-255 channel
// values — no guessing at the source string's shape.
const probeCanvas = document.createElement('canvas')
probeCanvas.width = 1
probeCanvas.height = 1
const probeCtx = probeCanvas.getContext('2d', { willReadFrequently: true })

const FALLBACK_HEX = '888888'
const HEX_PATTERN = /^[0-9a-f]{6}$/i

function resolveToHex(cssColor: string): string {
  if (!probeCtx) return FALLBACK_HEX

  probeCtx.fillStyle = cssColor
  probeCtx.fillRect(0, 0, 1, 1)
  const [r, g, b] = probeCtx.getImageData(0, 0, 1, 1).data

  const hex = [r, g, b].map((n) => n.toString(16).padStart(2, '0')).join('')

  // Belt-and-suspenders: canvas pixel channels are always clamped to
  // 0-255, so this should be unreachable, but Monaco throws on anything
  // outside \{6,8\} hex digits and a silently wrong editor theme is a much
  // worse failure mode than a flat gray fallback.
  if (!HEX_PATTERN.test(hex)) {
    console.warn(`monacoTheme: resolved "${cssColor}" to invalid hex "${hex}", using fallback`)
    return FALLBACK_HEX
  }

  return hex
}

function readToken(name: string): string {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return resolveToHex(raw)
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
