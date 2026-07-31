// The redesign's signature element: one bar per test case instead of a
// generic progress bar. Deliberately derived only from passed_count/
// total_test_cases — no per-test-case data exists in the API, and the
// judge worker is fail-fast (stops at the first failing case), so the
// state of every case is fully determined by those two numbers already:
// cases before passed_count passed, the case right after is what broke
// the run (unless the submission was accepted outright), and anything
// past that never ran. Bar height is uniform on purpose — inventing
// per-bar height variation would be exactly the synthetic/decorative
// data this derivation is meant to avoid.
type ExecutionTrailProps = {
  passedCount: number | null
  totalTestCases: number | null
  size?: 'sm' | 'lg'
}

type BarState = 'passed' | 'failed' | 'not-run'

export function ExecutionTrail({ passedCount, totalTestCases, size = 'lg' }: ExecutionTrailProps) {
  if (passedCount === null || totalTestCases === null || totalTestCases === 0) {
    return null
  }

  const accepted = passedCount === totalTestCases
  const bars: BarState[] = Array.from({ length: totalTestCases }, (_, i) => {
    if (i < passedCount) return 'passed'
    if (i === passedCount && !accepted) return 'failed'
    return 'not-run'
  })

  return (
    <span className={`execution-trail execution-trail--${size}`} aria-hidden="true">
      {bars.map((state, i) => (
        <span key={i} className={`execution-trail__bar execution-trail__bar--${state}`} />
      ))}
    </span>
  )
}
