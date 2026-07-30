import { useEffect, useRef, useState } from 'react'
import { Link, Navigate, useParams } from 'react-router-dom'

import { getSubmission, listChallenges, type Submission } from '../api'
import { TelemetryChip } from '../components/TelemetryChip'
import { VerdictBadge } from '../components/VerdictBadge'

const POLLING_DELAY_MS = 2000

export function SubmissionPage() {
  const { id, subId } = useParams<{ id: string; subId: string }>()
  const timeoutRef = useRef<number | null>(null)

  const [submission, setSubmission] = useState<Submission | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)
  // undefined = still resolving; null = determined, no next challenge;
  // string = the next challenge's id. Same display order as /desafios —
  // GET /challenges already returns challenges grouped by collection
  // ordinal then created_at, so "next" is simply the following array entry.
  const [nextChallengeId, setNextChallengeId] = useState<string | null | undefined>(undefined)

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
          clearPollingTimer()
          timeoutRef.current = window.setTimeout(() => {
            void fetchSubmission()
          }, POLLING_DELAY_MS)
          return
        }

        clearPollingTimer()
      } catch (err) {
        if (!active) {
          return
        }

        setError(err instanceof Error ? err.message : 'Failed to load submission')
        setLoading(false)
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

  useEffect(() => {
    let active = true
    if (!id) return

    listChallenges()
      .then((data) => {
        if (!active) return
        const index = data.findIndex((c) => c.id === id)
        setNextChallengeId(index !== -1 && index + 1 < data.length ? data[index + 1].id : null)
      })
      .catch(() => {
        if (active) setNextChallengeId(null)
      })

    return () => {
      active = false
    }
  }, [id])

  if (!id || !subId) {
    return <Navigate to="/" replace />
  }

  return (
    <main className="page-shell page-shell--submission">
      <Link className="back-link" to={`/challenges/${id}`}>
        ← Voltar ao desafio
      </Link>

      {loading ? <p className="status-message">Carregando submissão...</p> : null}

      {error ? <p className="status-message status-error">{error}</p> : null}

      {submission && submission.status === 'pending' ? (
        <section className="panel submission-panel submission-pending">
          <span className="submission-pending__spinner" aria-hidden="true" />
          <p className="submission-pending__text">Avaliando sua solução...</p>
        </section>
      ) : null}

      {submission && submission.status !== 'pending' ? (
        <section className="panel submission-panel">
          <p className="eyebrow">Submissão</p>
          <h1>Resultado</h1>

          <div className="submission-meta">
            <span className="submission-verdict-reveal">
              <VerdictBadge status={submission.status} />
            </span>
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
            ) : null}
          </div>

          {submission.status === 'accepted' && submission.stdout !== null ? (
            <section className="submission-output">
              <div className="submission-output__block">
                <p className="submission-output__label">Saída obtida</p>
                <pre className="submission-output__pre">{submission.stdout}</pre>
              </div>
            </section>
          ) : null}

          {submission.status === 'wrong_answer' && submission.stdout !== null ? (
            <section className="submission-output">
              <div className="submission-output__block">
                <p className="submission-output__label">Saída obtida</p>
                <pre className="submission-output__pre">{submission.stdout}</pre>
              </div>
              <div className="submission-output__block">
                <p className="submission-output__label">Saída esperada</p>
                <pre className="submission-output__pre">{submission.expected_output}</pre>
              </div>
            </section>
          ) : null}

          {(submission.status === 'runtime_error' || submission.status === 'time_limit_exceeded') &&
          submission.stderr !== null ? (
            <section className="submission-output">
              <div className="submission-output__block">
                <p className="submission-output__label">Erro</p>
                <pre className="submission-output__pre">{submission.stderr}</pre>
              </div>
            </section>
          ) : null}

          <section>
            <p className="muted">Código enviado</p>
            <pre className="submission-code">
              <code>{submission.source_code}</code>
            </pre>
          </section>

          {submission.status === 'accepted' ? (
            <div className="submission-next-action">
              {nextChallengeId ? (
                <Link className="challenge-submit" to={`/challenges/${nextChallengeId}`}>
                  Próximo desafio →
                </Link>
              ) : nextChallengeId === null ? (
                <button type="button" className="challenge-submit" disabled title="Não há mais desafios">
                  Não há mais desafios
                </button>
              ) : null}
            </div>
          ) : null}
        </section>
      ) : null}
    </main>
  )
}