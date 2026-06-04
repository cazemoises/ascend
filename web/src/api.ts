export const API_BASE_URL = 'http://localhost:8080'

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
  sample_test_cases: SampleTestCase[]
  created_at: string
  updated_at: string
}

export interface Submission {
  id: string
  challenge_id: string
  language: SubmissionLanguage
  source_code: string
  status: string
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
    throw new Error(message)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
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

export function getSubmission(submissionId: string) {
  return requestJSON<Submission>(`/api/v1/submissions/${submissionId}`)
}