import Editor from '@monaco-editor/react'
import { FormEvent, useEffect, useState } from 'react'
import { Link, Navigate, useNavigate, useParams } from 'react-router-dom'

import {
  createSubmission,
  getChallenge,
  listChallengeSubmissions,
  type Challenge,
  type CreateSubmissionRequest,
  type SubmissionLanguage,
  type SubmissionSummary,
} from '../api'

const CODE_TEMPLATES: Record<SubmissionLanguage, string> = {
  python: `import sys

def main():
    data = input().split()
    # TODO
    print()

if __name__ == '__main__':
    main()
`,
  go: `package main

import (
\t"bufio"
\t"fmt"
\t"os"
)

func main() {
\treader := bufio.NewReader(os.Stdin)
\tvar a, b int
\tfmt.Fscan(reader, &a, &b)
\t// TODO
\tfmt.Println()
}
`,
  javascript: `const lines = [];
process.stdin.on('data', d => lines.push(...d.toString().split('\\n')));
process.stdin.on('end', () => {
  // TODO
  console.log();
});
`,
}

export function ChallengePage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [challenge, setChallenge] = useState<Challenge | null>(null)
  const [language, setLanguage] = useState<SubmissionLanguage>('python')
  const [sourceCode, setSourceCode] = useState<string>('')
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState<boolean>(false)
  const [submissions, setSubmissions] = useState<SubmissionSummary[]>([])

  useEffect(() => {
    let active = true

    async function loadChallenge() {
      if (!id) {
        if (active) {
          setError('Missing challenge id')
          setLoading(false)
        }
        return
      }

      try {
        setLoading(true)
        setError(null)
        const data = await getChallenge(id)
        if (active) {
          setChallenge(data)
        }
      } catch (err) {
        if (active) {
          setError(err instanceof Error ? err.message : 'Failed to load challenge')
        }
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    }

    void loadChallenge()

    return () => {
      active = false
    }
  }, [id])

  useEffect(() => {
    if (!id) return
    listChallengeSubmissions(id)
      .then(setSubmissions)
      .catch(() => {})
  }, [id])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!id) {
      setError('Missing challenge id')
      return
    }

    try {
      setSubmitting(true)
      setError(null)

      const payload: CreateSubmissionRequest = {
        language,
        source_code: sourceCode,
      }

      const response = await createSubmission(id, payload)
      navigate(`/challenges/${id}/submissions/${response.submission_id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit solution')
    } finally {
      setSubmitting(false)
    }
  }

  if (!id) {
    return <Navigate to="/" replace />
  }

  return (
    <main className="page-shell">
      <Link className="back-link" to="/">
        ← Voltar aos desafios
      </Link>

      {loading ? <p className="status-message">Carregando desafio...</p> : null}

      {error ? <p className="status-message status-error">{error}</p> : null}

      {challenge ? (
        <section className="panel">
          <p className="eyebrow">Challenge</p>
          <h1>{challenge.title}</h1>
          <p className="muted">{challenge.description}</p>

          {challenge.notes ? (
            <p style={{
              margin: '16px 0',
              padding: '10px 14px',
              borderLeft: '3px solid var(--border)',
              background: 'var(--surface-alt, rgba(255,255,255,0.04))',
              fontSize: '0.875rem',
              whiteSpace: 'pre-wrap',
            }}>
              {challenge.notes}
            </p>
          ) : null}

          <div className="challenge-card__meta" style={{ marginTop: '16px', marginBottom: '24px' }}>
            <span className={`difficulty difficulty--${challenge.difficulty}`}>{challenge.difficulty}</span>
            <span className="challenge-card__slug">/{challenge.slug}</span>
            <span className="muted">time limit: {challenge.time_limit_ms}ms</span>
            <span className="muted">memory limit: {challenge.memory_limit_mb}MB</span>
          </div>

          {challenge.sample_test_cases.length > 0 ? (
            <div style={{ marginBottom: '24px' }}>
              <h2 style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '12px' }}>Exemplos</h2>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                <thead>
                  <tr>
                    <th style={{ textAlign: 'left', padding: '6px 12px', borderBottom: '1px solid var(--border)' }}>
                      Entrada
                    </th>
                    <th style={{ textAlign: 'left', padding: '6px 12px', borderBottom: '1px solid var(--border)' }}>
                      Saída Esperada
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {challenge.sample_test_cases.map((tc, i) => (
                    <tr key={i}>
                      <td style={{ padding: '6px 12px', verticalAlign: 'top' }}>
                        <pre style={{ margin: 0, fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>
                          {tc.input || '(vazio)'}
                        </pre>
                      </td>
                      <td style={{ padding: '6px 12px', verticalAlign: 'top' }}>
                        <pre style={{ margin: 0, fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>
                          {tc.expected_output}
                        </pre>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}

          <form onSubmit={handleSubmit}>
            <label className="muted" htmlFor="language">
              Language
            </label>
            <select
              id="language"
              value={language}
              onChange={(event) => {
                const lang = event.target.value as SubmissionLanguage
                setLanguage(lang)
                if (sourceCode === '') {
                  setSourceCode(CODE_TEMPLATES[lang])
                }
              }}
              className="challenge-input"
              disabled={submitting}
            >
              <option value="python">python</option>
              <option value="go">go</option>
              <option value="javascript">javascript</option>
            </select>

            <p className="muted" style={{ marginTop: '16px', marginBottom: '4px' }}>Source code</p>
            <Editor
              height="400px"
              theme="vs-dark"
              language={language}
              value={sourceCode}
              onChange={(v) => setSourceCode(v ?? '')}
              options={{ fontFamily: 'monospace', readOnly: submitting }}
            />

            <button type="submit" className="challenge-submit" disabled={submitting}>
              {submitting ? 'Submitting...' : 'Submit'}
            </button>
          </form>

          <div style={{ marginTop: '32px' }}>
            <h2 style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '12px' }}>Histórico de Submissões</h2>
            {submissions.length > 0 ? (
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                <thead>
                  <tr>
                    <th style={{ textAlign: 'left', padding: '6px 12px', borderBottom: '1px solid var(--border)' }}>Status</th>
                    <th style={{ textAlign: 'left', padding: '6px 12px', borderBottom: '1px solid var(--border)' }}>Linguagem</th>
                    <th style={{ textAlign: 'left', padding: '6px 12px', borderBottom: '1px solid var(--border)' }}>Data</th>
                  </tr>
                </thead>
                <tbody>
                  {submissions.map((sub) => (
                    <tr key={sub.id}>
                      <td style={{ padding: '6px 12px' }}>
                        <span className={`difficulty difficulty--${sub.status === 'accepted' ? 'easy' : sub.status === 'pending' ? 'medium' : 'hard'}`}>
                          {sub.status}
                        </span>
                      </td>
                      <td style={{ padding: '6px 12px' }}>{sub.language}</td>
                      <td style={{ padding: '6px 12px', color: 'var(--muted)' }}>
                        {new Date(sub.created_at).toLocaleString('pt-BR')}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <p className="muted">Sem submissões ainda.</p>
            )}
          </div>
        </section>
      ) : null}
    </main>
  )
}