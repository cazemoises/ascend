## ADDED Requirements

### Requirement: Editor pre-populates with a language-specific starter template
When the editor is empty and the user changes the language `<select>`, the Monaco Editor SHALL pre-populate with a minimal starter template for the selected language. The template SHALL provide the basic structure for reading from stdin and printing to stdout. If the editor already contains user-typed content, changing the language SHALL NOT overwrite the existing code.

#### Scenario: Template applied when editor is empty on language change
- **WHEN** the user has not typed anything (editor value is empty string) and selects a different language
- **THEN** the editor is populated with the starter template for that language

#### Scenario: Template not applied when editor has content
- **WHEN** the user has typed code into the editor and changes the language
- **THEN** the editor content is preserved unchanged

#### Scenario: Python template
- **WHEN** the editor is empty and the user selects "python"
- **THEN** the editor contains a Python starter snippet with `import sys` and `input()` usage

#### Scenario: Go template
- **WHEN** the editor is empty and the user selects "go"
- **THEN** the editor contains a Go starter snippet with `package main`, `import`, `func main()`, and `fmt.Scan` usage

#### Scenario: JavaScript template
- **WHEN** the editor is empty and the user selects "javascript"
- **THEN** the editor contains a JavaScript starter snippet reading from `process.stdin`
