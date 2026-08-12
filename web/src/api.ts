// Empty string = same-origin (production behind nginx); dev talks to the Go
// server directly.
export const API_BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

export type UserRole = 'student' | 'teacher'

export interface AuthUser {
  id: string
  email: string
  // real_role never changes for the session; effective_role reflects the
  // "Ver como aluno" override (see viewAs.ts) and is what the UI should
  // gate on everywhere except the toggle's own visibility.
  real_role: UserRole
  effective_role: UserRole
}

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export type ChallengeDifficulty = 'easy' | 'medium' | 'hard'
export const DIFFICULTY_LABELS: Record<ChallengeDifficulty, string> = {
  easy: 'Fácil',
  medium: 'Médio',
  hard: 'Difícil',
}
export type SubmissionLanguage = 'python' | 'go' | 'javascript' | 'sql' | 'java' | 'c' | 'cpp'
// A challenge's language: null keeps today's multi-language behavior (the
// student picks from every SubmissionLanguage). A non-null value pins the
// challenge to exactly that one — enforced generically server-side
// (store.languageAllowed: submission.language must equal challenge.language
// whenever the latter is set, not just for "sql"), so any SubmissionLanguage
// value is possible on read even though 'sql' is the only one the create/
// edit Studio form (ChallengeList.tsx's LanguageDraft) currently offers —
// e.g. the Trilha de Evolução challenges are pinned to "python" this way
// without going through that form. ChallengeLanguage stays scoped to just
// 'sql' for LanguageDraft/CreateChallengeInput below, matching what the API
// actually accepts on write; only the read-side Challenge.language field
// uses the full SubmissionLanguage union, since GET can return any pinned
// value regardless of what POST/PUT currently allow.
export type ChallengeLanguage = 'sql'

export interface SampleTestCase {
  input: string
  expected_output: string
  ordinal: number
}

export interface Challenge {
  id: string
  slug: string
  title: string
  description: string
  difficulty: ChallengeDifficulty
  time_limit_ms: number
  memory_limit_mb: number
  notes: string | null
  starter_code: string | null
  // Per-language override of starter_code: when a key is present for the
  // student's chosen language, its marker-delimited stub/harness is used
  // instead of starter_code (which only ever holds one language's harness).
  // Absent/undefined for every challenge that predates this field.
  starter_code_by_language?: Partial<Record<SubmissionLanguage, string>> | null
  language: SubmissionLanguage | null
  sql_schema: string | null
  collection_id: string | null
  sample_test_cases: SampleTestCase[]
  created_at: string
  updated_at: string
  // Whether the current viewer has an accepted, non-test-run submission for
  // this challenge. Present on GET /challenges and GET /challenges/:id;
  // absent on create/update responses.
  solved?: boolean
  // Join-derived display value, only present on GET /challenges (absent
  // elsewhere, and null on that endpoint too when collection_id is null).
  collection_title?: string | null
  // Join-derived (through the challenge's collection) — a challenge has no
  // group_id column of its own. Same presence/null rules as
  // collection_title: only on GET /challenges, null when there's no group.
  group_id?: string | null
  group_title?: string | null
}

export interface ChallengeCollection {
  id: string
  teacher_id: string
  title: string
  ordinal: number
  group_id: string | null
  created_at: string
  updated_at: string
}

// No updated_at — unlike ChallengeCollection, nothing about a group after
// creation is ever surfaced with a last-modified timestamp.
export interface ChallengeGroup {
  id: string
  teacher_id: string
  title: string
  ordinal: number
  created_at: string
}

export interface SubmissionSummary {
  id: string
  status: string
  language: SubmissionLanguage
  is_test_run: boolean
  created_at: string
}

export interface Submission {
  id: string
  challenge_id: string
  language: SubmissionLanguage
  source_code: string
  status: string
  exec_time_ms: number | null
  memory_peak_mb: number | null
  stderr: string | null
  // Only non-null on a wrong_answer verdict — the failing case's actual vs.
  // expected output.
  stdout: string | null
  expected_output: string | null
  // failed_input is only non-null when the failing case is a sample
  // (failed_is_sample true) — a hidden case's input is never sent by the
  // backend at all, not just hidden here. failed_is_sample is tri-state:
  // false means "hidden case, show a notice instead"; null means "this
  // submission predates the column, show neither the input nor the notice"
  // — those two read the same on failed_input alone, so check this first.
  failed_input: string | null
  failed_is_sample: boolean | null
  passed_count: number | null
  total_test_cases: number | null
  time_limit_ms: number
  memory_limit_mb: number
  created_at: string
  updated_at: string
}

export interface CreateSubmissionRequest {
  language: SubmissionLanguage
  source_code: string
}

export interface CreateSubmissionResponse {
  submission_id: string
}

// "Ver como aluno": a real teacher's own client-side UI preference, never a
// security boundary (the backend independently ignores X-View-As from
// anyone whose real role isn't teacher). Persisted so it survives reloads.
const VIEW_AS_STORAGE_KEY = 'ascend:viewAsStudent'

export function getViewAsStudent(): boolean {
  return localStorage.getItem(VIEW_AS_STORAGE_KEY) === 'true'
}

export function setViewAsStudent(value: boolean): void {
  if (value) {
    localStorage.setItem(VIEW_AS_STORAGE_KEY, 'true')
  } else {
    localStorage.removeItem(VIEW_AS_STORAGE_KEY)
  }
}

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  if (init?.body != null && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (getViewAsStudent()) {
    headers.set('X-View-As', 'student')
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers,
  })

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`
    try {
      const payload = (await response.json()) as { error?: string }
      if (payload.error) {
        message = payload.error
      }
    } catch {
      const text = await response.text()
      if (text) {
        message = text
      }
    }
    throw new ApiError(response.status, message)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

export interface CreateChallengeInput {
  slug: string
  title: string
  description: string
  difficulty: ChallengeDifficulty
  time_limit_ms?: number
  memory_limit_mb?: number
  notes?: string | null
  starter_code?: string | null
  starter_code_by_language?: Partial<Record<SubmissionLanguage, string>> | null
  language?: ChallengeLanguage | null
  sql_schema?: string | null
  collection_id?: string | null
}

export function createChallenge(body: CreateChallengeInput) {
  return requestJSON<Omit<Challenge, 'sample_test_cases'>>('/api/v1/challenges', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export interface CreateTestCaseInput {
  input: string
  expected_output: string
  is_sample: boolean
  order_matters: boolean
}

export interface TestCase extends CreateTestCaseInput {
  id: string
  challenge_id: string
  ordinal: number
}

export function createTestCase(challengeId: string, body: CreateTestCaseInput) {
  return requestJSON<TestCase>(`/api/v1/challenges/${challengeId}/test-cases`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateChallenge(id: string, body: CreateChallengeInput) {
  return requestJSON<Omit<Challenge, 'sample_test_cases'>>(`/api/v1/challenges/${id}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export interface ImportTestCaseInput {
  input?: string
  expected_output: string
  is_sample?: boolean
  ordinal?: number
  order_matters?: boolean
}

export interface ImportChallengeInput extends CreateChallengeInput {
  test_cases?: ImportTestCaseInput[]
}

export interface ImportChallengesInput {
  challenges: ImportChallengeInput[]
  // Links every challenge in the batch to this collection (matched
  // case-insensitively against the teacher's own collections, created if
  // none matches) — a whole import normally belongs to one collection.
  collection_title?: string | null
}

export interface ChallengeWithTestCases extends Omit<Challenge, 'sample_test_cases'> {
  test_cases: TestCase[]
}

export function importChallenges(body: ImportChallengesInput) {
  return requestJSON<ChallengeWithTestCases[]>('/api/v1/challenges/import', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function listTestCases(challengeId: string) {
  return requestJSON<TestCase[]>(`/api/v1/challenges/${challengeId}/test-cases`)
}

export function listChallengeCollections() {
  return requestJSON<ChallengeCollection[]>('/api/v1/challenge-collections')
}

export interface ChallengeCollectionInput {
  title: string
  // Omitted on create means "next available ordinal"; required on update
  // (the backend defaults a missing value to 0, which would silently reset
  // the collection's position) — always pass the collection's current
  // ordinal when only renaming it.
  ordinal?: number
  // null means "no group". Same rule as ordinal: on update, always pass
  // the collection's current group_id when not deliberately changing it,
  // or omitting it would silently clear the group.
  group_id?: string | null
}

export function createChallengeCollection(body: ChallengeCollectionInput) {
  return requestJSON<ChallengeCollection>('/api/v1/challenge-collections', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateChallengeCollection(id: string, body: Required<ChallengeCollectionInput>) {
  return requestJSON<ChallengeCollection>(`/api/v1/challenge-collections/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function listChallengeGroups() {
  return requestJSON<ChallengeGroup[]>('/api/v1/challenge-groups')
}

export interface ChallengeGroupInput {
  title: string
  // Same "omitted on create = next available, required on update" rule as
  // ChallengeCollectionInput.ordinal.
  ordinal?: number
}

export function createChallengeGroup(body: ChallengeGroupInput) {
  return requestJSON<ChallengeGroup>('/api/v1/challenge-groups', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateChallengeGroup(id: string, body: Required<ChallengeGroupInput>) {
  return requestJSON<ChallengeGroup>(`/api/v1/challenge-groups/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function reorderChallengeGroups(items: { id: string; ordinal: number }[]) {
  return requestJSON<void>('/api/v1/challenge-groups/reorder', {
    method: 'PATCH',
    body: JSON.stringify({ items }),
  })
}

export function reorderChallengeCollections(items: { id: string; ordinal: number }[]) {
  return requestJSON<void>('/api/v1/challenge-collections/reorder', {
    method: 'PATCH',
    body: JSON.stringify({ items }),
  })
}

export function replaceTestCases(challengeId: string, testCases: CreateTestCaseInput[]) {
  return requestJSON<TestCase[]>(`/api/v1/challenges/${challengeId}/test-cases`, {
    method: 'PUT',
    body: JSON.stringify({ test_cases: testCases }),
  })
}

// fetchMe resolves the caller's identity from the Remote-Email header the
// Pangolin proxy sets on every request — there is no login form or token.
// Rejects with a 401 ApiError when Pangolin (or DEV_FAKE_EMAIL locally)
// didn't identify a caller.
export function fetchMe() {
  return requestJSON<AuthUser>('/api/v1/auth/me')
}

// Walks every page of GET /api/v1/challenges (server caps limit at 100) and
// concatenates them — ChallengeList renders the whole catalog with no
// pagination UI of its own, so it needs every challenge regardless of how
// many pages that takes, not just the first page's default 50.
export async function listChallenges() {
  const pageSize = 100
  const all: Challenge[] = []
  let offset = 0
  for (;;) {
    const page = await requestJSON<Challenge[]>(
      `/api/v1/challenges?limit=${pageSize}&offset=${offset}`,
    )
    all.push(...page)
    if (page.length < pageSize) break
    offset += pageSize
  }
  return all
}

export function getChallenge(id: string) {
  return requestJSON<Challenge>(`/api/v1/challenges/${id}`)
}

export function createSubmission(challengeId: string, body: CreateSubmissionRequest) {
  return requestJSON<CreateSubmissionResponse>(`/api/v1/challenges/${challengeId}/submissions`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export interface UserSubmission {
  id: string
  challenge_id: string
  challenge_title: string
  language: SubmissionLanguage
  status: string
  is_test_run: boolean
  created_at: string
}

export interface SubmissionHistoryResponse {
  items: UserSubmission[]
  next_cursor: string | null
}

export function listMySubmissions(cursor?: string, limit = 20) {
  const params = new URLSearchParams({ limit: String(limit) })
  if (cursor !== undefined) {
    params.set('cursor', cursor)
  }
  return requestJSON<SubmissionHistoryResponse>(`/api/v1/submissions?${params.toString()}`)
}

export function getSubmission(submissionId: string) {
  return requestJSON<Submission>(`/api/v1/submissions/${submissionId}`)
}

export function listChallengeSubmissions(challengeId: string) {
  return requestJSON<SubmissionSummary[]>(`/api/v1/challenges/${challengeId}/submissions`)
}

export interface LastSubmissionCode {
  language: SubmissionLanguage
  source_code: string
}

// 404s when the caller has never submitted to this challenge — callers
// should catch and fall back to starter_code, not surface it as an error.
export function getLastSubmission(challengeId: string) {
  return requestJSON<LastSubmissionCode>(`/api/v1/challenges/${challengeId}/submissions/last`)
}

export interface StatsBucket {
  up_to_ms: number
  count: number
}

export interface ChallengeStats {
  total_runs: number
  user_best_ms: number | null
  faster_than_pct: number | null
  buckets: StatsBucket[]
}

export function getChallengeStats(challengeId: string) {
  return requestJSON<ChallengeStats>(`/api/v1/challenges/${challengeId}/stats`)
}

export interface SubmissionAttempt {
  submission_id: string
  verdict: string
  created_at: string
  language: string
  exec_time_ms: number | null
}

export interface StudentChallengeSummary {
  challenge_id: string
  challenge_title: string
  attempts: SubmissionAttempt[]
  total_attempts: number
  accepted_count: number
  wrong_answer_count: number
  runtime_error_count: number
  timeout_count: number
}

export interface StudentStats {
  total_submissions: number
  challenges_attempted: number
  challenges_accepted: number
  avg_attempts_to_solve: number | null
}

export interface StudentListCompletion {
  list_title: string
  item_title: string
  completed_at: string
}

export interface StudentOverview {
  student_id: string
  email: string
  stats: StudentStats
  challenges: StudentChallengeSummary[]
  list_completions: StudentListCompletion[]
}

export function getStudentsOverview() {
  return requestJSON<StudentOverview[]>('/api/v1/teacher/students-overview')
}

export type ListItemDifficulty = 'easy' | 'medium' | 'hard'

export interface ProblemList {
  id: string
  teacher_id: string
  title: string
  week_label: string | null
  week_start: string | null
  week_end: string | null
  description: string | null
  published: boolean
  created_at: string
  updated_at: string
  // Only present on the GET /lists collection response — derived server-side
  // from week_start/week_end against today, not persisted. Mutually
  // exclusive: a list is never both.
  is_current?: boolean
  is_upcoming?: boolean
}

export interface ListItem {
  id: string
  list_id: string
  ordinal: number
  title: string
  difficulty: ListItemDifficulty
  is_bonus: boolean
  body: string
  // Only meaningful for a student viewer; null for a teacher's own view. For
  // an item with linked_challenge_id set, this is derived automatically from
  // the student having solved that challenge, not a self-declared checkbox.
  completed: boolean | null
  // When set, this item's completion is automatic — resolve the challenge at
  // /challenges/{linked_challenge_id} instead of self-declaring via
  // complete/uncomplete (the API rejects those calls for a linked item).
  linked_challenge_id: string | null
  challenge_title: string | null
  challenge_slug: string | null
  created_at: string
  updated_at: string
}

export interface ProblemListDetail extends ProblemList {
  items: ListItem[]
}

export interface CreateProblemListInput {
  title: string
  week_label?: string | null
  week_start?: string | null
  week_end?: string | null
  description?: string | null
}

export interface UpdateProblemListInput {
  title: string
  week_label?: string | null
  week_start?: string | null
  week_end?: string | null
  description?: string | null
  published: boolean
}

export function listProblemLists() {
  return requestJSON<ProblemList[]>('/api/v1/lists')
}

export function getProblemList(id: string) {
  return requestJSON<ProblemListDetail>(`/api/v1/lists/${id}`)
}

export function createProblemList(body: CreateProblemListInput) {
  return requestJSON<ProblemList>('/api/v1/lists', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateProblemList(id: string, body: UpdateProblemListInput) {
  return requestJSON<ProblemList>(`/api/v1/lists/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function deleteProblemList(id: string) {
  return requestJSON<void>(`/api/v1/lists/${id}`, { method: 'DELETE' })
}

export interface ImportListItemInput {
  title: string
  difficulty: ListItemDifficulty
  is_bonus?: boolean
  body: string
}

export interface ImportProblemListInput {
  title: string
  week_label?: string | null
  week_start?: string | null
  week_end?: string | null
  description?: string | null
  items: ImportListItemInput[]
}

export function importProblemList(body: ImportProblemListInput) {
  return requestJSON<ProblemListDetail>('/api/v1/lists/import', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export interface CreateListItemInput {
  title: string
  difficulty: ListItemDifficulty
  is_bonus: boolean
  body: string
  // When set, this item's completion is derived from the student having
  // solved that challenge instead of a self-declared checkbox.
  linked_challenge_id?: string | null
}

export function createListItem(listId: string, body: CreateListItemInput) {
  return requestJSON<ListItem>(`/api/v1/lists/${listId}/items`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateListItem(itemId: string, body: CreateListItemInput) {
  return requestJSON<ListItem>(`/api/v1/list-items/${itemId}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function deleteListItem(itemId: string) {
  return requestJSON<void>(`/api/v1/list-items/${itemId}`, { method: 'DELETE' })
}

export function reorderListItems(listId: string, items: { item_id: string; ordinal: number }[]) {
  return requestJSON<void>(`/api/v1/lists/${listId}/reorder`, {
    method: 'PATCH',
    body: JSON.stringify({ items }),
  })
}

export function completeListItem(itemId: string) {
  return requestJSON<void>(`/api/v1/list-items/${itemId}/complete`, { method: 'POST' })
}

export function uncompleteListItem(itemId: string) {
  return requestJSON<void>(`/api/v1/list-items/${itemId}/complete`, { method: 'DELETE' })
}