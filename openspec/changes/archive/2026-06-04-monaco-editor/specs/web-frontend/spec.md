## MODIFIED Requirements

### Requirement: Production build succeeds
Running `npm run build` SHALL produce a static bundle in `web/dist/` with no TypeScript or build errors. The bundle SHALL include the Monaco Editor web workers bundled by Vite automatically via `@monaco-editor/react`.

#### Scenario: Build output exists
- **WHEN** `npm run build` completes successfully
- **THEN** `web/dist/index.html` exists and the exit code is 0

#### Scenario: Monaco workers are included in the build
- **WHEN** `npm run build` completes successfully
- **THEN** the `web/dist/` directory contains Monaco worker assets and no worker-related errors appear in the browser console
