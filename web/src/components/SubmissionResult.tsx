import type { ReactNode } from 'react'
import type { Submission } from '../api'
import { HTTPTestDebugger } from './HTTPTestDebugger'
import { ExecutionTrail } from './ExecutionTrail'
import { TelemetryChip } from './TelemetryChip'
import { VerdictBadge } from './VerdictBadge'

// Same 12px icon convention as TelemetryChip's own IconClock/IconMemory —
// these two extend that chip row to cover language and test count too.
function IconLanguage() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <polyline points="16 18 22 12 16 6" />
      <polyline points="8 6 2 12 8 18" />
    </svg>
  )
}

function IconTestCount() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <polyline points="9 11 12 14 22 4" />
      <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
    </svg>
  )
}

export function SubmissionResult({ submission, children }: { submission: Submission; children?: ReactNode }) {
  return (
    <section className="panel submission-panel">
      <div className={`submission-verdict-hero submission-verdict-hero--${submission.status}`}>
        <div className="submission-verdict-hero__row">
          <ExecutionTrail
            passedCount={submission.passed_count}
            totalTestCases={submission.total_test_cases}
            size="lg"
          />
          <span className="submission-verdict-reveal">
            <VerdictBadge status={submission.status} />
          </span>
        </div>

        <div className="submission-meta">
          <TelemetryChip
            timeLimitMs={submission.time_limit_ms}
            memoryLimitMb={submission.memory_limit_mb}
          />
          <span className="submission-meta__item">
            <IconLanguage />
            {submission.language}
          </span>
          {submission.passed_count !== null && submission.total_test_cases !== null ? (
            <span className="submission-meta__item">
              <IconTestCount />
              {submission.passed_count} de {submission.total_test_cases} casos
            </span>
          ) : null}
        </div>
      </div>

      <div className="submission-panel__body">
        {submission.language === 'fastapi' ? <HTTPTestDebugger key={submission.id} submission={submission}/> : null}
        {submission.language !== 'fastapi' && submission.status === 'accepted' && submission.stdout !== null ? (
          <section className="submission-section">
            <div className="submission-output">
              <div className="result-panel">
                <p className="result-panel__header">Saída obtida</p>
                <pre className="result-panel__body">{submission.stdout}</pre>
              </div>
            </div>
          </section>
        ) : null}

        {submission.language !== 'fastapi' && submission.status === 'wrong_answer' && submission.failed_input !== null ? (
          <section className="submission-section">
            <div className="result-panel">
              <p className="result-panel__header">Entrada</p>
              <pre className="result-panel__body">{submission.failed_input}</pre>
            </div>
          </section>
        ) : null}

        {submission.language !== 'fastapi' && submission.status === 'wrong_answer' &&
        submission.failed_input === null &&
        submission.failed_is_sample === false ? (
          <section className="submission-section">
            <p className="status-message">Este caso de teste é oculto.</p>
          </section>
        ) : null}

        {submission.language !== 'fastapi' && submission.status === 'wrong_answer' && submission.stdout !== null ? (
          <section className="submission-section">
            <div className="submission-output">
              <div className="result-panel result-panel--actual">
                <p className="result-panel__header">Saída obtida</p>
                <pre className="result-panel__body">{submission.stdout}</pre>
              </div>
              <div className="result-panel result-panel--expected">
                <p className="result-panel__header">Saída esperada</p>
                <pre className="result-panel__body">{submission.expected_output}</pre>
              </div>
            </div>
          </section>
        ) : null}

        {submission.language !== 'fastapi' && (submission.status === 'runtime_error' || submission.status === 'time_limit_exceeded') &&
        submission.stderr !== null ? (
          <section className="submission-section">
            <div className="submission-output">
              <div className="result-panel">
                <p className="result-panel__header">Logs e diagnósticos</p>
                <pre className="result-panel__body">{submission.stderr}</pre>
              </div>
            </div>
          </section>
        ) : null}

        {children}
      </div>
    </section>
  )
}
