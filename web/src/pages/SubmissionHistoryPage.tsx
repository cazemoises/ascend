import { Fragment, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { getSubmission, listMySubmissions, type Submission, type UserSubmission } from '../api'
import { useAuth } from '../auth/useAuth'
import { VerdictBadge } from '../components/VerdictBadge'

const STDERR_STATUSES = new Set(['runtime_error', 'compile_error', 'error'])

export function SubmissionHistoryPage() {
  const { isTeacher } = useAuth()
  const [items, setItems] = useState<UserSubmission[]>([])
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [pageCursor, setPageCursor] = useState<string | undefined>(undefined)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Audit pane: which row is open, plus a cache of fetched details so
  // re-opening a row never refetches.
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [details, setDetails] = useState<Record<string, Submission>>({})
  const [detailLoading, setDetailLoading] = useState<string | null>(null)

  useEffect(() => {
    let active = true

    async function loadPage() {
      try {
        const page = await listMySubmissions(pageCursor)
        if (!active) {
          return
        }
        setItems((prev) => (pageCursor === undefined ? page.items : [...prev, ...page.items]))
        setNextCursor(page.next_cursor)
        setError(null)
      } catch (err) {
        if (active) {
          setError(err instanceof Error ? err.message : 'Falha ao carregar submissões')
        }
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    }

    void loadPage()

    return () => {
      active = false
    }
  }, [pageCursor])

  async function toggleAudit(id: string) {
    if (expandedId === id) {
      setExpandedId(null)
      return
    }
    setExpandedId(id)
    if (details[id] === undefined) {
      setDetailLoading(id)
      try {
        const sub = await getSubmission(id)
        setDetails((prev) => ({ ...prev, [id]: sub }))
      } catch {
        // Row stays expanded with the error fallback below.
      } finally {
        setDetailLoading((prev) => (prev === id ? null : prev))
      }
    }
  }

  return (
    <main className="page-shell">
      <section className="hero">
        <p className="eyebrow">Ascend</p>
        <h1>minhas submissões</h1>
        <p className="muted">
          Histórico das suas soluções, da mais recente para a mais antiga. Clique em um veredito
          para auditar o código, o desempenho e os logs daquela tentativa.
        </p>
      </section>

      {error ? <p className="status-message status-error">{error}</p> : null}

      {!error ? (
        <div className="datagrid">
          <table>
            <thead>
              <tr>
                <th>Veredito</th>
                <th>Desafio</th>
                <th>Linguagem</th>
                <th>Data</th>
              </tr>
            </thead>
            <tbody>
              {items.map((sub) => {
                const detail: Submission | undefined = details[sub.id]
                const isOpen = expandedId === sub.id
                return (
                  <Fragment key={sub.id}>
                    <tr>
                      <td>
                        <button
                          type="button"
                          className="audit-toggle"
                          aria-expanded={isOpen}
                          onClick={() => void toggleAudit(sub.id)}
                        >
                          <span
                            className={isOpen ? 'audit-caret audit-caret--open' : 'audit-caret'}
                            aria-hidden="true"
                          >
                            ▸
                          </span>
                          <VerdictBadge status={sub.status} />
                          {isTeacher && sub.is_test_run ? (
                            <span className="verdict test-run-tag">TESTE</span>
                          ) : null}
                        </button>
                      </td>
                      <td>
                        <Link to={`/challenges/${sub.challenge_id}/submissions/${sub.id}`}>
                          {sub.challenge_title}
                        </Link>
                      </td>
                      <td>{sub.language}</td>
                      <td className="muted">{new Date(sub.created_at).toLocaleString('pt-BR')}</td>
                    </tr>
                    {isOpen ? (
                      <tr className="audit-row">
                        <td colSpan={4}>
                          {detailLoading === sub.id ? (
                            <p className="muted audit-loading">Carregando telemetria...</p>
                          ) : detail !== undefined ? (
                            <div className="audit">
                              <div className="audit__metrics">
                                <span className="constraint">
                                  Tempo de execução{' '}
                                  <strong>
                                    {detail.exec_time_ms !== null
                                      ? `${detail.exec_time_ms}ms`
                                      : '—'}{' '}
                                    / {detail.time_limit_ms}ms
                                  </strong>
                                </span>
                                <span className="constraint">
                                  Memória de pico{' '}
                                  <strong>
                                    {detail.memory_peak_mb !== null
                                      ? `${detail.memory_peak_mb}MB`
                                      : '—'}{' '}
                                    / {detail.memory_limit_mb}MB
                                  </strong>
                                </span>
                                <span className="constraint">
                                  Linguagem <strong>{detail.language}</strong>
                                </span>
                              </div>
                              <pre className="submission-code">
                                <code>{detail.source_code}</code>
                              </pre>
                              {detail.stderr !== null && STDERR_STATUSES.has(detail.status) ? (
                                <div className="terminal-log">
                                  <div className="terminal-log__bar">
                                    <span aria-hidden="true" />
                                    <span aria-hidden="true" />
                                    <span aria-hidden="true" />
                                    stderr
                                  </div>
                                  <pre>{detail.stderr}</pre>
                                </div>
                              ) : null}
                            </div>
                          ) : (
                            <p className="status-message status-error">
                              Falha ao carregar os detalhes desta submissão.
                            </p>
                          )}
                        </td>
                      </tr>
                    ) : null}
                  </Fragment>
                )
              })}
              {!loading && items.length === 0 ? (
                <tr>
                  <td colSpan={4} className="datagrid__empty">
                    Nenhuma submissão ainda — escolha um desafio e envie sua primeira solução.
                  </td>
                </tr>
              ) : null}
              {loading ? (
                <tr>
                  <td colSpan={4} className="datagrid__empty">
                    Carregando...
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      ) : null}

      {nextCursor !== null && !loading ? (
        <button
          type="button"
          className="load-more"
          onClick={() => {
            setLoading(true)
            setPageCursor(nextCursor)
          }}
        >
          carregar mais
        </button>
      ) : null}
    </main>
  )
}
