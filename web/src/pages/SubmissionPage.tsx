import { SubmissionResult } from '../components/SubmissionResult'
import { useEffect, useRef, useState } from 'react'
import { Link, Navigate, useParams } from 'react-router-dom'

import { getLiveSession, getSubmission, listChallenges, type Challenge, type Submission } from '../api'
import { playAcceptedChime } from '../lib/sound'

const POLLING_DELAY_MS = 2000

// GET /challenges returns challenges sorted by collection ordinal then
// created_at — NOT the alphabetical order /desafios actually displays them
// in (its default sort since the "default challenge list sort to
// alphabetical" change). Numbered titles like "01. ...", "02. ..." only
// land in that order by title, not by creation time, so "next" must
// re-sort each collection's own challenges alphabetically before walking
// the list — otherwise "next" can jump to whichever challenge in the
// collection happened to be created second, skipping the ones in between.
// Collection (and therefore group) order itself is left untouched: once a
// collection's challenges are exhausted, the next entry is already the
// first challenge of the following collection in the server's order.
function orderForNextChallenge(challenges: Challenge[]): Challenge[] {
  const buckets = new Map<string, Challenge[]>()
  const collectionOrder: string[] = []
  for (const challenge of challenges) {
    const key = challenge.collection_id ?? ''
    let bucket = buckets.get(key)
    if (!bucket) {
      bucket = []
      buckets.set(key, bucket)
      collectionOrder.push(key)
    }
    bucket.push(challenge)
  }
  for (const bucket of buckets.values()) {
    bucket.sort((a, b) => a.title.localeCompare(b.title))
  }
  return collectionOrder.flatMap((key) => buckets.get(key) ?? [])
}

export function SubmissionPage() {
  const { id, subId } = useParams<{ id: string; subId: string }>()
  const liveSessionId = new URLSearchParams(window.location.search).get('liveSessionId')
  const timeoutRef = useRef<number | null>(null)

  const [submission, setSubmission] = useState<Submission | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)
  // undefined = still resolving; null = determined, no next challenge;
  // string = the next challenge's id. See orderForNextChallenge — walks
  // each collection in its own alphabetical (title) order, matching what
  // /desafios actually displays, not the raw API order.
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

    const challenges = liveSessionId
      ? getLiveSession(liveSessionId).then((data) => data.session.items.map((item) => ({ id: item.linked_challenge_id ?? '', title: item.title } as Challenge)))
      : listChallenges()

    challenges
      .then((data) => {
        if (!active) return
        const ordered = orderForNextChallenge(data)
        const index = ordered.findIndex((c) => c.id === id)
        setNextChallengeId(index !== -1 && index + 1 < ordered.length ? ordered[index + 1].id : null)
      })
      .catch(() => {
        if (active) setNextChallengeId(null)
      })

    return () => {
      active = false
    }
  }, [id, liveSessionId])

  if (!id || !subId) {
    return <Navigate to="/" replace />
  }

  return (
    <main className="page-shell page-shell--submission">
      <Link className="back-link" to={liveSessionId ? `/sessoes/${liveSessionId}` : `/challenges/${id}`}>
        ← voltar ao desafio
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
        <>
          <p className="eyebrow">Submissão</p>
          <h1>resultado</h1>

          <SubmissionResult submission={submission}>
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
                      <Link className="challenge-submit" to={`/challenges/${nextChallengeId}${liveSessionId ? `?liveSessionId=${liveSessionId}` : ''}`}>
                        próximo desafio →
                      </Link>
                    ) : nextChallengeId === null ? (
                      <button type="button" className="challenge-submit" disabled title="não há mais desafios">
                        não há mais desafios
                      </button>
                    ) : null}
                  </div>
                </section>
              ) : null}
          </SubmissionResult>
        </>
      ) : null}
    </main>
  )
}
