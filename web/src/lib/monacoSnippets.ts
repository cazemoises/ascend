import type { Monaco } from '@monaco-editor/react'
import type { editor, Position } from 'monaco-editor'

// Investigated before writing this: Monaco's own defaults already cover
// word-based suggestions (wordBasedSuggestions: 'matchingDocuments') and
// quick-suggest-as-you-type (quickSuggestions: {other:'on'}) for every
// language, no config needed — confirmed by reading
// node_modules/monaco-editor's own editorConfigurationSchema.js rather than
// assuming. What's actually missing is syntax snippets: grepping
// basic-languages/{python,go,java,cpp,javascript}/*.js for
// registerCompletionItemProvider turns up nothing — the basic-languages
// bundle only ships tokenizers (syntax highlighting) and language
// configuration (brackets/comments), never completion items. Even
// javascript, which does get real semantic completions from Monaco's
// bundled TypeScript language service, doesn't get typing "for" ->
// full-loop-skeleton the way a VS Code snippets extension would — that's a
// separate contribution VS Code ships, not part of the monaco-editor
// package. So every one of these languages needed an explicit snippet
// provider registered, not a config flag flipped.
type SnippetSpec = { label: string; insertText: string; detail: string }

const SNIPPETS: Partial<Record<string, SnippetSpec[]>> = {
  python: [
    { label: 'for', detail: 'for loop', insertText: 'for ${1:i} in range(${2:n}):\n\t$0' },
    { label: 'if', detail: 'if statement', insertText: 'if ${1:condition}:\n\t$0' },
    { label: 'while', detail: 'while loop', insertText: 'while ${1:condition}:\n\t$0' },
    { label: 'def', detail: 'function definition', insertText: 'def ${1:name}(${2:args}):\n\t$0' },
    { label: 'main', detail: '__main__ guard', insertText: "if __name__ == '__main__':\n\t$0" },
  ],
  go: [
    {
      label: 'for',
      detail: 'for loop',
      insertText: 'for ${1:i} := 0; ${1:i} < ${2:n}; ${1:i}++ {\n\t$0\n}',
    },
    { label: 'if', detail: 'if statement', insertText: 'if ${1:condition} {\n\t$0\n}' },
    {
      label: 'func',
      detail: 'function definition',
      insertText: 'func ${1:name}(${2:params}) ${3:returnType} {\n\t$0\n}',
    },
    { label: 'err', detail: 'error check', insertText: 'if err != nil {\n\t$0\n}' },
    { label: 'main', detail: 'main function', insertText: 'func main() {\n\t$0\n}' },
  ],
  javascript: [
    {
      label: 'for',
      detail: 'for loop',
      insertText: 'for (let ${1:i} = 0; ${1:i} < ${2:n}; ${1:i}++) {\n\t$0\n}',
    },
    { label: 'if', detail: 'if statement', insertText: 'if (${1:condition}) {\n\t$0\n}' },
    { label: 'while', detail: 'while loop', insertText: 'while (${1:condition}) {\n\t$0\n}' },
    {
      label: 'function',
      detail: 'function definition',
      insertText: 'function ${1:name}(${2:params}) {\n\t$0\n}',
    },
    { label: 'log', detail: 'console.log', insertText: 'console.log($0);' },
  ],
  java: [
    {
      label: 'for',
      detail: 'for loop',
      insertText: 'for (int ${1:i} = 0; ${1:i} < ${2:n}; ${1:i}++) {\n\t$0\n}',
    },
    { label: 'if', detail: 'if statement', insertText: 'if (${1:condition}) {\n\t$0\n}' },
    { label: 'while', detail: 'while loop', insertText: 'while (${1:condition}) {\n\t$0\n}' },
    {
      label: 'main',
      detail: 'main method',
      insertText: 'public static void main(String[] args) {\n\t$0\n}',
    },
    { label: 'sout', detail: 'System.out.println', insertText: 'System.out.println($0);' },
  ],
  c: [
    {
      label: 'for',
      detail: 'for loop',
      insertText: 'for (int ${1:i} = 0; ${1:i} < ${2:n}; ${1:i}++) {\n\t$0\n}',
    },
    { label: 'if', detail: 'if statement', insertText: 'if (${1:condition}) {\n\t$0\n}' },
    { label: 'while', detail: 'while loop', insertText: 'while (${1:condition}) {\n\t$0\n}' },
    { label: 'main', detail: 'main function', insertText: 'int main(void) {\n\t$0\n\treturn 0;\n}' },
    { label: 'printf', detail: 'printf', insertText: 'printf("$1\\n"$0);' },
    { label: 'scanf', detail: 'scanf', insertText: 'scanf("$1", $0);' },
  ],
  cpp: [
    {
      label: 'for',
      detail: 'for loop',
      insertText: 'for (int ${1:i} = 0; ${1:i} < ${2:n}; ${1:i}++) {\n\t$0\n}',
    },
    { label: 'if', detail: 'if statement', insertText: 'if (${1:condition}) {\n\t$0\n}' },
    { label: 'while', detail: 'while loop', insertText: 'while (${1:condition}) {\n\t$0\n}' },
    { label: 'main', detail: 'main function', insertText: 'int main() {\n\t$0\n\treturn 0;\n}' },
    { label: 'cout', detail: 'std::cout', insertText: 'std::cout << $0 << std::endl;' },
    { label: 'cin', detail: 'std::cin', insertText: 'std::cin >> $0;' },
  ],
}

// monaco.languages.registerCompletionItemProvider is a global registration
// against the shared monaco namespace, not scoped to one Editor instance —
// calling this more than once (e.g. ChallengePage re-mounting as a student
// navigates between challenges) would otherwise stack duplicate providers
// and show every snippet twice. Guarded the same shape defineAscendMonacoTheme
// doesn't need (monaco.editor.defineTheme overwrites by name, so it's
// naturally idempotent) but registerCompletionItemProvider isn't.
let registered = false

export function registerAscendSnippets(monaco: Monaco): void {
  if (registered) return
  registered = true

  for (const [language, snippets] of Object.entries(SNIPPETS)) {
    if (!snippets) continue
    monaco.languages.registerCompletionItemProvider(language, {
      provideCompletionItems(model: editor.ITextModel, position: Position) {
        const word = model.getWordUntilPosition(position)
        const range = {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        }
        return {
          suggestions: snippets.map((s) => ({
            label: s.label,
            kind: monaco.languages.CompletionItemKind.Snippet,
            detail: s.detail,
            insertText: s.insertText,
            insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
            range,
          })),
        }
      },
    })
  }

  registerJavaMemberCompletions(monaco)
}

// Cheaper alternative to a real language server (see
// docs/superpowers/plans/2026-08-10-lsp-java-autocomplete.md — full jdtls
// was evaluated and rejected: no official Docker image, 300MB+ per active
// session, a whole new backend subsystem). This is a static, curated map of
// the JDK types this judge's Java challenges actually use to their common
// instance methods, plus a source-scan heuristic to guess which type a
// variable is — not real type inference (doesn't survive chained calls
// like `texto.trim().` or generics), just enough to make `.` after a
// `String`/`Scanner`/`StringBuilder` variable suggest its real methods
// instead of only whatever identifiers already appear in the buffer.
// Scope approved 2026-08-10: these three types only (the most common across
// the 41 trilha-* Java challenges); List/Map/LinkedList deferred to a
// second pass, not blocking.
const JAVA_MEMBER_TYPES = ['String', 'Scanner', 'StringBuilder'] as const

const JAVA_MEMBERS: Record<(typeof JAVA_MEMBER_TYPES)[number], SnippetSpec[]> = {
  String: [
    { label: 'length', detail: 'int length()', insertText: 'length()' },
    { label: 'charAt', detail: 'char charAt(int index)', insertText: 'charAt(${1:index})' },
    { label: 'substring', detail: 'String substring(int beginIndex)', insertText: 'substring(${1:beginIndex})' },
    {
      label: 'substring',
      detail: 'String substring(int beginIndex, int endIndex)',
      insertText: 'substring(${1:beginIndex}, ${2:endIndex})',
    },
    { label: 'indexOf', detail: 'int indexOf(String str)', insertText: 'indexOf(${1:str})' },
    { label: 'lastIndexOf', detail: 'int lastIndexOf(String str)', insertText: 'lastIndexOf(${1:str})' },
    { label: 'contains', detail: 'boolean contains(CharSequence s)', insertText: 'contains(${1:s})' },
    { label: 'startsWith', detail: 'boolean startsWith(String prefix)', insertText: 'startsWith(${1:prefix})' },
    { label: 'endsWith', detail: 'boolean endsWith(String suffix)', insertText: 'endsWith(${1:suffix})' },
    { label: 'toUpperCase', detail: 'String toUpperCase()', insertText: 'toUpperCase()' },
    { label: 'toLowerCase', detail: 'String toLowerCase()', insertText: 'toLowerCase()' },
    { label: 'trim', detail: 'String trim()', insertText: 'trim()' },
    { label: 'strip', detail: 'String strip()', insertText: 'strip()' },
    {
      label: 'replace',
      detail: 'String replace(CharSequence target, CharSequence replacement)',
      insertText: 'replace(${1:target}, ${2:replacement})',
    },
    { label: 'split', detail: 'String[] split(String regex)', insertText: 'split(${1:regex})' },
    { label: 'equals', detail: 'boolean equals(Object obj)', insertText: 'equals(${1:obj})' },
    {
      label: 'equalsIgnoreCase',
      detail: 'boolean equalsIgnoreCase(String other)',
      insertText: 'equalsIgnoreCase(${1:other})',
    },
    { label: 'isEmpty', detail: 'boolean isEmpty()', insertText: 'isEmpty()' },
    { label: 'isBlank', detail: 'boolean isBlank()', insertText: 'isBlank()' },
    { label: 'toCharArray', detail: 'char[] toCharArray()', insertText: 'toCharArray()' },
    { label: 'concat', detail: 'String concat(String str)', insertText: 'concat(${1:str})' },
    { label: 'compareTo', detail: 'int compareTo(String other)', insertText: 'compareTo(${1:other})' },
    { label: 'matches', detail: 'boolean matches(String regex)', insertText: 'matches(${1:regex})' },
    { label: 'repeat', detail: 'String repeat(int count)', insertText: 'repeat(${1:count})' },
  ],
  Scanner: [
    { label: 'nextInt', detail: 'int nextInt()', insertText: 'nextInt()' },
    { label: 'nextLong', detail: 'long nextLong()', insertText: 'nextLong()' },
    { label: 'nextDouble', detail: 'double nextDouble()', insertText: 'nextDouble()' },
    { label: 'next', detail: 'String next()', insertText: 'next()' },
    { label: 'nextLine', detail: 'String nextLine()', insertText: 'nextLine()' },
    { label: 'nextBoolean', detail: 'boolean nextBoolean()', insertText: 'nextBoolean()' },
    { label: 'hasNext', detail: 'boolean hasNext()', insertText: 'hasNext()' },
    { label: 'hasNextInt', detail: 'boolean hasNextInt()', insertText: 'hasNextInt()' },
    { label: 'hasNextLine', detail: 'boolean hasNextLine()', insertText: 'hasNextLine()' },
    { label: 'close', detail: 'void close()', insertText: 'close()' },
  ],
  StringBuilder: [
    { label: 'append', detail: 'StringBuilder append(String str)', insertText: 'append(${1:str})' },
    { label: 'toString', detail: 'String toString()', insertText: 'toString()' },
    { label: 'reverse', detail: 'StringBuilder reverse()', insertText: 'reverse()' },
    {
      label: 'insert',
      detail: 'StringBuilder insert(int offset, String str)',
      insertText: 'insert(${1:offset}, ${2:str})',
    },
    { label: 'deleteCharAt', detail: 'StringBuilder deleteCharAt(int index)', insertText: 'deleteCharAt(${1:index})' },
    {
      label: 'delete',
      detail: 'StringBuilder delete(int start, int end)',
      insertText: 'delete(${1:start}, ${2:end})',
    },
    { label: 'length', detail: 'int length()', insertText: 'length()' },
    { label: 'charAt', detail: 'char charAt(int index)', insertText: 'charAt(${1:index})' },
    {
      label: 'setCharAt',
      detail: 'void setCharAt(int index, char ch)',
      insertText: 'setCharAt(${1:index}, ${2:ch})',
    },
    { label: 'indexOf', detail: 'int indexOf(String str)', insertText: 'indexOf(${1:str})' },
    {
      label: 'replace',
      detail: 'StringBuilder replace(int start, int end, String str)',
      insertText: 'replace(${1:start}, ${2:end}, ${3:str})',
    },
  ],
}

// Scans everything in the buffer before the cursor for a declaration or
// parameter of one of JAVA_MEMBER_TYPES naming this exact variable —
// `String texto = ...`, `Scanner scanner = ...`, or a method parameter like
// `(String texto)` all match. Source-order only (a declaration after the
// use site won't be found), which matches how these single-file student
// submissions actually read.
function inferJavaVariableType(
  model: editor.ITextModel,
  position: Position,
  varName: string,
): (typeof JAVA_MEMBER_TYPES)[number] | undefined {
  const textBefore = model.getValueInRange({
    startLineNumber: 1,
    startColumn: 1,
    endLineNumber: position.lineNumber,
    endColumn: position.column,
  })
  const declRe = new RegExp(`\\b(${JAVA_MEMBER_TYPES.join('|')})\\s+${varName}\\s*[=;,)]`)
  const match = declRe.exec(textBefore)
  return match?.[1] as (typeof JAVA_MEMBER_TYPES)[number] | undefined
}

function registerJavaMemberCompletions(monaco: Monaco): void {
  monaco.languages.registerCompletionItemProvider('java', {
    triggerCharacters: ['.'],
    provideCompletionItems(model: editor.ITextModel, position: Position) {
      const lineUpToCursor = model.getValueInRange({
        startLineNumber: position.lineNumber,
        startColumn: 1,
        endLineNumber: position.lineNumber,
        endColumn: position.column,
      })
      const varMatch = /([A-Za-z_]\w*)\.$/.exec(lineUpToCursor)
      if (!varMatch) return { suggestions: [] }

      const type = inferJavaVariableType(model, position, varMatch[1])
      const members = type ? JAVA_MEMBERS[type] : undefined
      if (!members) return { suggestions: [] }

      const range = {
        startLineNumber: position.lineNumber,
        endLineNumber: position.lineNumber,
        startColumn: position.column,
        endColumn: position.column,
      }
      return {
        suggestions: members.map((m) => ({
          label: m.label,
          kind: monaco.languages.CompletionItemKind.Method,
          detail: m.detail,
          insertText: m.insertText,
          insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
          range,
        })),
      }
    },
  })
}
