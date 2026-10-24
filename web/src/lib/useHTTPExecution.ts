import { useEffect, useRef, useState } from 'react'
import { createSubmission, getSubmission, type HTTPRequest, type Submission } from '../api'

export function useHTTPExecution(challengeID?: string, liveSessionID?: string, identity?: string) {
  const [tab, setTab] = useState<'client' | 'results'>('client')
  const [submission, setSubmission] = useState<Submission | null>(null)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const generation = useRef(0)
  useEffect(() => {
    const guard = generation
    const current = ++guard.current
    queueMicrotask(() => {
      if (current !== generation.current) return
      setSubmission(null); setPending(false); setError(null); setTab('client')
    })
    return () => { guard.current++ }
  }, [challengeID, liveSessionID, identity])

  async function run(source: string, requests?: HTTPRequest[]) {
    if (!challengeID || pending) return
    const current = ++generation.current
    setTab('results'); setPending(true); setError(null); setSubmission(null)
    try {
      const created = await createSubmission(challengeID, {
        language: 'fastapi', source_code: source,
        ...(requests ? { http_requests: requests } : {}),
        ...(liveSessionID ? { live_session_id: liveSessionID } : {}),
      })
      while (current === generation.current) {
        const result = await getSubmission(created.submission_id)
        if (current !== generation.current) return
        setSubmission(result)
        if (result.status !== 'pending') break
        await new Promise(resolve => window.setTimeout(resolve, 1000))
      }
    } catch (cause) {
      if (current === generation.current) setError(cause instanceof Error ? cause.message : 'Falha na execução')
    } finally {
      if (current === generation.current) setPending(false)
    }
  }
  return { submission, pending, error, run, tab, setTab }
}
