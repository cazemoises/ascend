## ADDED Requirements

### Requirement: Vite dev server starts and serves the SPA
The frontend SHALL be a Vite + React + TypeScript project. Running `npm run dev` SHALL start a dev server on port 5173 (default) and serve the application.

#### Scenario: Dev server starts
- **WHEN** `npm run dev` is run in `web/`
- **THEN** the Vite dev server starts and prints a local URL

#### Scenario: TypeScript strict mode enabled
- **WHEN** the project is compiled
- **THEN** `strict: true` is set in `tsconfig.json` and the build succeeds with no type errors

### Requirement: API calls proxied to the backend in development
The Vite dev config SHALL proxy requests with the `/api/` prefix to `http://localhost:8080` so the frontend can call the API without CORS issues during development.

#### Scenario: Proxied API request
- **WHEN** the frontend fetches `/api/v1/healthz` in development
- **THEN** Vite forwards the request to `http://localhost:8080/api/v1/healthz`

### Requirement: Placeholder home page renders without errors
The SPA SHALL render a placeholder home page at `/` with the application name "Ascend" and no runtime errors in the browser console.

#### Scenario: Home page renders
- **WHEN** a user navigates to `/`
- **THEN** a page with the text "Ascend" is displayed and the browser console shows no errors

### Requirement: Production build succeeds
Running `npm run build` SHALL produce a static bundle in `web/dist/` with no TypeScript or build errors.

#### Scenario: Build output exists
- **WHEN** `npm run build` completes successfully
- **THEN** `web/dist/index.html` exists and the exit code is 0
