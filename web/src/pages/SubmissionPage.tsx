import { useEffect, useRef, useState } from 'react'
import { Link, Navigate, useParams } from 'react-router-dom'

import { getSubmission, type Submission } from '../api'
import { TelemetryChip } from '../components/TelemetryChip'
import { VerdictBadge } from '../components/VerdictBadge'

const POLLING_DELAY_MS = 2000

export function SubmissionPage() {
  const { id, subId } = useParams<{ id: string; subId: string }>()
  const timeoutRef = useRef<number | null>(null)

  const [submission, setSubmission] = useState<Submission | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)
  const [polling, setPolling] = useState<boolean>(false)

  useEffect(() => {
    let active = true

    function clearPollingTimer() {
      if (timeoutRef.current !== null) {
        window.clearTimeout(timeoutRef.current)
        timeoutRef.current = null
      }
    }

    async function fetchSubmission() {
      if (!subId) {
        if (active) {
          setError('Missing submission id')
          setLoading(false)
        }
        return
      }

      try {
        if (active) {
          setError(null)
        }

        const data = await getSubmission(subId)

        if (!active) {
          return
        }

        setSubmission(data)
        setLoading(false)

        if (data.status === 'pending') {
          setPolling(true)
          clearPollingTimer()
          timeoutRef.current = window.setTimeout(() => {
            void fetchSubmission()
          }, POLLING_DELAY_MS)
          return
        }

        setPolling(false)
        clearPollingTimer()
      } catch (err) {
        if (!active) {
          return
        }

        setError(err instanceof Error ? err.message : 'Failed to load submission')
        setLoading(false)
        setPolling(false)
        clearPollingTimer()
      }
    }

    void fetchSubmission()

    return () => {
      active = false
      if (timeoutRef.current !== null) {
        window.clearTimeout(timeoutRef.current)
        timeoutRef.current = null
      }
    }
  }, [subId])

  if (!id || !subId) {
    return <Navigate to="/" replace />
  }

  return (
    <main className="page-shell">
      <Link className="back-link" to={`/challenges/${id}`}>
        ← Voltar ao desafio
      </Link>

      {loading ? <p className="status-message">Carregando submissão...</p> : null}

      {error ? <p className="status-message status-error">{error}</p> : null}

      {submission ? (
        <section className="panel submission-panel">
          <p className="eyebrow">Submissão</p>
          <h1>Resultado</h1>

          <div className="submission-meta">
            <VerdictBadge status={submission.status} />
            <TelemetryChip
              timeLimitMs={submission.time_limit_ms}
              memoryLimitMb={submission.memory_limit_mb}
            />
            <span className="submission-meta__item">{submission.language}</span>
            <span className="submission-meta__item">#{submission.id.slice(0, 8)}</span>
            {submission.passed_count !== null && submission.total_test_cases !== null ? (
              <span className="submission-meta__item">
                {submission.passed_count} de {submission.total_test_cases} test cases passaram
              </span>
            ) : polling ? (
              <span className="submission-meta__item">atualizando...</span>
            ) : null}
          </div>

          <section>
            <p className="muted">Código enviado</p>
            <pre className="submission-code">
              <code>{submission.source_code}</code>
            </pre>
          </section>
        </section>
      ) : null}
    </main>
  )
}