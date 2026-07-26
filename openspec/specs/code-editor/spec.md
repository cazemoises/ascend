# Spec: Code Editor

## Purpose

TBD — Monaco Editor integration for the challenge submission form. Provides a rich, syntax-highlighted code editing experience in place of a plain textarea.

## Requirements

### Requirement: Monaco Editor renders on the challenge page
The challenge submission form SHALL render a Monaco Editor component (`@monaco-editor/react`) in place of the plain textarea. The editor SHALL display with a dark theme (`vs-dark`), a fixed height of `400px`, and a monospace font family.

#### Scenario: Editor renders with correct appearance
- **WHEN** a user navigates to `/challenges/:id`
- **THEN** a Monaco Editor is displayed (not a `<textarea>`) with a dark background, 400px height, and monospace font

#### Scenario: Editor shows a placeholder / loading state
- **WHEN** the Monaco workers have not yet finished loading
- **THEN** a loading indicator is shown in the editor area and no error is thrown

### Requirement: Editor value is synchronized with submission state
The editor SHALL be a controlled component. Its value SHALL be the `sourceCode` React state. Every change in the editor SHALL update `sourceCode` so the submission handler receives the current editor content without any additional step.

#### Scenario: Typing in the editor updates submission state
- **WHEN** a user types code into the Monaco Editor
- **THEN** the `sourceCode` state reflects the editor content at the time of form submission

#### Scenario: Submitting uses editor content
- **WHEN** a user clicks Submit after typing code
- **THEN** the POST body contains `source_code` equal to what was typed in the editor

### Requirement: Language mode updates with the language select
When the user changes the language `<select>`, the Monaco Editor's language mode SHALL update immediately to match: `python` → `python`, `go` → `go`, `javascript` → `javascript`.

#### Scenario: Switching language updates syntax highlighting
- **WHEN** a user selects "go" from the language dropdown
- **THEN** the Monaco Editor applies Go syntax highlighting without a page reload

#### Scenario: Default language on load
- **WHEN** a user navigates to the challenge page
- **THEN** the editor defaults to `python` language mode (matching the default select value)

### Requirement: Editor is disabled during submission
While a submission is in progress (`submitting === true`), the editor SHALL be set to `readOnly` mode so the user cannot modify the code being submitted.

#### Scenario: Editor locked during submit
- **WHEN** the user clicks Submit and the request is in flight
- **THEN** the Monaco Editor is in read-only mode and the language select is disabled
