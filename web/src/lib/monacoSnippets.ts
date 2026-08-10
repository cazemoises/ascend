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
// the JDK types most common in algorithm/data-structure exercises to their
// common instance methods, plus a source-scan heuristic to guess which type
// a variable is — not real type inference (doesn't survive chained calls
// like `texto.trim().`), just enough to make `.` after a variable of one of
// these types suggest its real methods instead of only whatever identifiers
// already appear in the buffer.
//
// This is NOT "every Java type" — it's the curated short list that actually
// shows up across the 41 trilha-* challenges plus the general-purpose types
// (List/Map/Set) any algorithm exercise reaches for. A class the STUDENT
// defines (e.g. `FilaAtendimento` in trilha-n3-fila-fifo) will never get
// member completion through this mechanism — there's no static list to look
// it up in, and there never will be without a real language server (already
// evaluated and rejected on cost). That's an accepted, permanent limitation
// of this approach, not a gap to fill in a future pass.
//
// Scope: String/Scanner/StringBuilder approved 2026-08-10; List/ArrayList,
// Map/HashMap, Set/HashSet, Integer/Double statics, and the array `.length`
// field added in this pass. LinkedList still deferred (uses the same List
// surface students actually reach for elsewhere, so New Value Small — can
// come back if a real challenge needs it explicitly).
const JAVA_MEMBER_TYPES = [
  'String',
  'Scanner',
  'StringBuilder',
  'List',
  'ArrayList',
  'Map',
  'HashMap',
  'Set',
  'HashSet',
] as const

// List and Map and Set are commonly declared by their interface type
// (`List<String> nomes = new ArrayList<>();`) as often as by their concrete
// class — both spellings need to resolve to the same member list, since a
// student calling `.add` doesn't care which one they wrote on the left.
const LIST_MEMBERS: SnippetSpec[] = [
  { label: 'add', detail: 'boolean add(E element)', insertText: 'add(${1:element})' },
  { label: 'get', detail: 'E get(int index)', insertText: 'get(${1:index})' },
  { label: 'size', detail: 'int size()', insertText: 'size()' },
  { label: 'remove', detail: 'E remove(int index)', insertText: 'remove(${1:index})' },
  { label: 'contains', detail: 'boolean contains(Object o)', insertText: 'contains(${1:o})' },
  { label: 'indexOf', detail: 'int indexOf(Object o)', insertText: 'indexOf(${1:o})' },
  { label: 'isEmpty', detail: 'boolean isEmpty()', insertText: 'isEmpty()' },
  { label: 'set', detail: 'E set(int index, E element)', insertText: 'set(${1:index}, ${2:element})' },
]

const MAP_MEMBERS: SnippetSpec[] = [
  { label: 'put', detail: 'V put(K key, V value)', insertText: 'put(${1:key}, ${2:value})' },
  { label: 'get', detail: 'V get(Object key)', insertText: 'get(${1:key})' },
  { label: 'containsKey', detail: 'boolean containsKey(Object key)', insertText: 'containsKey(${1:key})' },
  { label: 'keySet', detail: 'Set<K> keySet()', insertText: 'keySet()' },
  { label: 'values', detail: 'Collection<V> values()', insertText: 'values()' },
  { label: 'entrySet', detail: 'Set<Map.Entry<K,V>> entrySet()', insertText: 'entrySet()' },
  { label: 'remove', detail: 'V remove(Object key)', insertText: 'remove(${1:key})' },
  { label: 'size', detail: 'int size()', insertText: 'size()' },
  {
    label: 'getOrDefault',
    detail: 'V getOrDefault(Object key, V defaultValue)',
    insertText: 'getOrDefault(${1:key}, ${2:defaultValue})',
  },
]

const SET_MEMBERS: SnippetSpec[] = [
  { label: 'add', detail: 'boolean add(E element)', insertText: 'add(${1:element})' },
  { label: 'contains', detail: 'boolean contains(Object o)', insertText: 'contains(${1:o})' },
  { label: 'remove', detail: 'boolean remove(Object o)', insertText: 'remove(${1:o})' },
  { label: 'size', detail: 'int size()', insertText: 'size()' },
  { label: 'isEmpty', detail: 'boolean isEmpty()', insertText: 'isEmpty()' },
]

const JAVA_MEMBERS: Record<(typeof JAVA_MEMBER_TYPES)[number], SnippetSpec[]> = {
  List: LIST_MEMBERS,
  ArrayList: LIST_MEMBERS,
  Map: MAP_MEMBERS,
  HashMap: MAP_MEMBERS,
  Set: SET_MEMBERS,
  HashSet: SET_MEMBERS,
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

// Static methods called directly on the type name — `Integer.parseInt(s)`,
// not on an instance — so these need no declaration lookup at all: if the
// identifier right before the `.` is literally one of these keys, that's
// the whole match. Deliberately narrow (only the statics asked for): a
// boxed Integer/Double INSTANCE's own methods (intValue(), compareTo(),
// ...) are out of scope here, not part of this request.
const JAVA_STATIC_MEMBERS: Partial<Record<string, SnippetSpec[]>> = {
  Integer: [
    { label: 'parseInt', detail: 'static int parseInt(String s)', insertText: 'parseInt(${1:s})' },
    { label: 'toString', detail: 'static String toString(int i)', insertText: 'toString(${1:i})' },
    { label: 'valueOf', detail: 'static Integer valueOf(String s)', insertText: 'valueOf(${1:s})' },
  ],
  Double: [
    { label: 'parseDouble', detail: 'static double parseDouble(String s)', insertText: 'parseDouble(${1:s})' },
    { label: 'toString', detail: 'static String toString(double d)', insertText: 'toString(${1:d})' },
    { label: 'valueOf', detail: 'static Double valueOf(String s)', insertText: 'valueOf(${1:s})' },
  ],
}

// `.length` is a FIELD on every array type (`int[]`, `String[]`, a custom
// type's array, doesn't matter), not a method — no parentheses, and no
// per-element-type list to look up since every array has exactly this one
// member. Handled as its own case rather than folded into JAVA_MEMBERS,
// which is keyed by declared type name and has no entry that means "any
// array of anything."
const ARRAY_LENGTH_FIELD: SnippetSpec = {
  label: 'length',
  detail: 'int length (campo do array, sem parênteses)',
  insertText: 'length',
}

// Scans everything in the buffer before the cursor for a declaration or
// parameter of one of JAVA_MEMBER_TYPES naming this exact variable —
// `String texto = ...`, `Scanner scanner = ...`, `List<String> nomes = ...`,
// or a method parameter like `(String texto)` all match. The optional
// `(?:<[\s\S]*?>)?` segment skips over generic type arguments (including
// nested ones like `Map<String, List<Integer>>`) without trying to parse
// them for real. Source-order only (a declaration after the use site won't
// be found), which matches how these single-file student submissions
// actually read.
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
  const declRe = new RegExp(`\\b(${JAVA_MEMBER_TYPES.join('|')})(?:<[\\s\\S]*?>)?\\s+${varName}\\s*[=;,)]`)
  const match = declRe.exec(textBefore)
  return match?.[1] as (typeof JAVA_MEMBER_TYPES)[number] | undefined
}

// Same shape as inferJavaVariableType but for `SomeType[] varName` — the
// element type is intentionally unconstrained (`\w+`, not JAVA_MEMBER_TYPES)
// since `.length` applies identically to an array of anything, including
// types this curated map has never heard of (e.g. `FilaAtendimento[]`).
function isJavaArrayVariable(model: editor.ITextModel, position: Position, varName: string): boolean {
  const textBefore = model.getValueInRange({
    startLineNumber: 1,
    startColumn: 1,
    endLineNumber: position.lineNumber,
    endColumn: position.column,
  })
  const declRe = new RegExp(`\\b\\w+(?:<[\\s\\S]*?>)?\\[\\]\\s+${varName}\\s*[=;,)]`)
  return declRe.test(textBefore)
}

function toSuggestions(
  monaco: Monaco,
  members: SnippetSpec[],
  kind: (typeof monaco.languages.CompletionItemKind)[keyof typeof monaco.languages.CompletionItemKind],
  range: { startLineNumber: number; endLineNumber: number; startColumn: number; endColumn: number },
) {
  return members.map((m) => ({
    label: m.label,
    kind,
    detail: m.detail,
    insertText: m.insertText,
    insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
    range,
  }))
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
      const varName = varMatch[1]

      const range = {
        startLineNumber: position.lineNumber,
        endLineNumber: position.lineNumber,
        startColumn: position.column,
        endColumn: position.column,
      }

      // Static call on the type name itself (`Integer.parseInt(...)`) takes
      // priority — it's an exact match on the identifier, not a guess.
      const staticMembers = JAVA_STATIC_MEMBERS[varName]
      if (staticMembers) {
        return { suggestions: toSuggestions(monaco, staticMembers, monaco.languages.CompletionItemKind.Method, range) }
      }

      if (isJavaArrayVariable(model, position, varName)) {
        return { suggestions: toSuggestions(monaco, [ARRAY_LENGTH_FIELD], monaco.languages.CompletionItemKind.Field, range) }
      }

      const type = inferJavaVariableType(model, position, varName)
      const members = type ? JAVA_MEMBERS[type] : undefined
      if (!members) return { suggestions: [] }

      return { suggestions: toSuggestions(monaco, members, monaco.languages.CompletionItemKind.Method, range) }
    },
  })
}
