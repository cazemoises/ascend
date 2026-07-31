import { useEffect, useRef, useState } from 'react'
import { Link, Navigate, useParams } from 'react-router-dom'

import { getSubmission, listChallenges, type Submission } from '../api'
import { TelemetryChip } from '../components/TelemetryChip'
import { VerdictBadge } from '../components/VerdictBadge'
import { playAcceptedChime } from '../lib/sound'

const POLLING_DELAY_MS = 2000

function IconCheck() {
  return (
    <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  )
}

function IconAlert() {
  return (
    <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  )
}

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
  // Tracks which submission id we've already chimed for, so the sound
  // plays once per accepted result — not on every re-render — even under
  // React StrictMode's double-invoked effects in dev.
  const chimedForRef = useRef<string | null>(null)

  useEffect(() => {
    if (submission?.status === 'accepted' && chimedForRef.current !== submission.id) {
      chimedForRef.current = submission.id
      playAcceptedChime()
    }
  }, [submission])

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

          <section className="submission-section">
            <p className="submission-section-label">Veredito</p>
            <div
              className={
                submission.status === 'accepted'
                  ? 'submission-verdict-hero submission-verdict-hero--accepted'
                  : 'submission-verdict-hero submission-verdict-hero--rejected'
              }
            >
              <div className="submission-verdict-hero__row">
                <span className="submission-verdict-hero__icon">
                  {submission.status === 'accepted' ? <IconCheck /> : <IconAlert />}
                </span>
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
          </section>

          {submission.status === 'accepted' && submission.stdout !== null ? (
            <section className="submission-section">
              <div className="submission-output">
                <div className="result-panel">
                  <p className="result-panel__header">Saída obtida</p>
                  <pre className="result-panel__body">{submission.stdout}</pre>
                </div>
              </div>
            </section>
          ) : null}

          {submission.status === 'wrong_answer' && submission.stdout !== null ? (
            <section className="submission-section">
              <div className="submission-output">
                <div className="result-panel">
                  <p className="result-panel__header">Saída obtida</p>
                  <pre className="result-panel__body">{submission.stdout}</pre>
                </div>
                <div className="result-panel">
                  <p className="result-panel__header">Saída esperada</p>
                  <pre className="result-panel__body">{submission.expected_output}</pre>
                </div>
              </div>
            </section>
          ) : null}

          {(submission.status === 'runtime_error' || submission.status === 'time_limit_exceeded') &&
          submission.stderr !== null ? (
            <section className="submission-section">
              <div className="submission-output">
                <div className="result-panel">
                  <p className="result-panel__header">Erro</p>
                  <pre className="result-panel__body">{submission.stderr}</pre>
                </div>
              </div>
            </section>
          ) : null}

          <section className="submission-section">
            <div className="result-panel result-panel--code">
              <p className="result-panel__header">Código enviado</p>
              <pre className="result-panel__body">
                <code>{submission.source_code}</code>
              </pre>
            </div>
          </section>

          {submission.status === 'accepted' ? (
            <section className="submission-section">
              <p className="submission-section-label">Ações</p>
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
            </section>
          ) : null}
        </section>
      ) : null}
    </main>
  )
}