import type { Submission } from '../api'

// Do not infer permission from is_test_run: mentors' graded runs also set it.
export function httpDebuggerView(submission: Submission) {
  if (submission.language !== 'fastapi' || submission.status === 'pending') return null
  if (submission.http_exploration) return { kind: 'exploration' as const, actual: submission.stdout, expected: null, comparison: undefined, logs: submission.stderr }
  if (submission.failed_is_sample === true && submission.http_debug && submission.stdout && submission.expected_output) {
    return { kind: 'comparison' as const, actual: submission.stdout, expected: submission.expected_output, comparison: submission.http_debug, logs: submission.stderr }
  }
  if (submission.http_feedback_safe) return { kind: 'response' as const, actual: submission.stdout, expected: null, comparison: undefined, logs: submission.stderr }
  // Legacy, malformed, hidden and state-tainted snapshots fail closed.
  return { kind: 'unavailable' as const, actual: null, expected: null, comparison: undefined, logs: null }
}
