// Empty string = same-origin (production behind nginx); dev talks to the Go
// server directly.
export const API_BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

const TOKEN_KEY = 'ascend_token'
const USER_KEY = 'ascend_user'
export const UNAUTHORIZED_EVENT = 'ascend:unauthorized'

export interface AuthUser {
  id: string
  email: string
  created_at: string
  updated_at: string
}

export interface AuthResponse {
  token: string
  user: AuthUser
}

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function getStoredUser(): AuthUser | null {
  const raw = localStorage.getItem(USER_KEY)
  if (raw === null) {
    return null
  }
  try {
    return JSON.parse(raw) as AuthUser
  } catch {
    return null
  }
}

export function storeSession(session: AuthResponse): void {
  localStorage.setItem(TOKEN_KEY, session.token)
  localStorage.setItem(USER_KEY, JSON.stringify(session.user))
}

export function clearSession(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
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

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  if (init?.body != null && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const token = getToken()
  if (token !== null && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers,
  })

  if (!response.ok) {
    // Expired/invalid session: clear once and notify AuthContext so route
    // guards redirect declaratively. Fires at most once — follow-up requests
    // carry no token and skip this branch. Auth endpoints are exempt: a
    // failed login must not destroy an existing session.
    if (response.status === 401 && token !== null && !path.startsWith('/api/v1/auth/')) {
      clearSession()
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    }
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

export function registerUser(email: string, password: string) {
  return requestJSON<AuthResponse>('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export function loginUser(email: string, password: string) {
  return requestJSON<AuthResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
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
  // from week_start/week_end against today, not persisted.
  is_current?: boolean
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