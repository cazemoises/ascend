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
export type SubmissionLanguage = 'python' | 'go' | 'javascript'

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
  class_id: string | null
  sample_test_cases: SampleTestCase[]
  created_at: string
  updated_at: string
  // Only meaningful on the GET /challenges feed: whether the current viewer
  // has an accepted, non-test-run submission for this challenge. Absent on
  // other endpoints (create/update/get single challenge).
  solved?: boolean
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
  class_id?: string | null
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

export function listTestCases(challengeId: string) {
  return requestJSON<TestCase[]>(`/api/v1/challenges/${challengeId}/test-cases`)
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

export function listChallenges() {
  return requestJSON<Challenge[]>('/api/v1/challenges')
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

export interface ClassScore {
  class_id: string
  class_name: string
  student_id: string
  student_email: string
  completed: number
  total: number
}

export function getTeacherScoreboard() {
  return requestJSON<ClassScore[]>('/api/v1/classes/scoreboard')
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
  // Only meaningful for a student viewer; null for a teacher's own view.
  completed: boolean | null
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