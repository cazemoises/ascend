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
}
