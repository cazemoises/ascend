import Editor from '@monaco-editor/react'
import { useEffect, useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate, useParams } from 'react-router-dom'

import {
  createSubmission,
  getChallenge,
  getChallengeStats,
  getLastSubmission,
  listChallengeSubmissions,
  type Challenge,
  type ChallengeStats,
  type CreateSubmissionRequest,
  type SubmissionLanguage,
  type SubmissionSummary,
} from '../api'
import { useAuth } from '../auth/useAuth'
import { VerdictBadge } from '../components/VerdictBadge'

// SQL challenges have no starter template or tab — the student writes the
// whole query from scratch, so its entries here are unused placeholders that
// exist only to satisfy Record<SubmissionLanguage, string>.
const CODE_TEMPLATES: Record<SubmissionLanguage, string> = {
  sql: '',
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

// Mirrors the judge worker's split-marker convention: in starter_code, the
// stub above the marker line is what the student sees and edits; everything
// below it is the hidden stdin/stdout harness the backend concatenates under
// the submission at execution time. The student never sees the harness.
const RUNNER_MARKER = '[[ASCEND::RUNNER]]'

function visibleTemplate(starter: string): string {
  const idx = starter.indexOf(RUNNER_MARKER)
  if (idx === -1) {
    return starter
  }
  // Cut back to the start of the marker line so its comment prefix
  // (# / //) never leaks into the editor.
  const head = starter.slice(0, idx)
  const lineStart = head.lastIndexOf('\n')
  const stub = (lineStart === -1 ? '' : head.slice(0, lineStart)).replace(/\s+$/, '')
  return stub === '' ? '' : `${stub}\n`
}

// SQL is deliberately excluded: it has no tab of its own (see the editor
// toolbar below), so it never needs a template swap or a tab label.
const LANGUAGES: SubmissionLanguage[] = ['python', 'go', 'javascript']

const FILE_NAMES: Record<SubmissionLanguage, string> = {
  sql: 'query.sql',
  python: 'solution.py',
  go: 'main.go',
  javascript: 'solution.js',
}

export function ChallengePage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { isTeacher } = useAuth()

  const [challenge, setChallenge] = useState<Challenge | null>(null)
  const [language, setLanguage] = useState<SubmissionLanguage>('python')
  const [sourceCode, setSourceCode] = useState<string>(CODE_TEMPLATES['python'])
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState<boolean>(false)
  const [submissions, setSubmissions] = useState<SubmissionSummary[]>([])
  const [stats, setStats] = useState<ChallengeStats | null>(null)

  useEffect(() => {
    let active = true

    async function loadChallenge() {
      if (!id) {
        if (active) {
          setError('Desafio não encontrado')
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
          if (data.language === 'sql') {
            // SQL challenges have no harness/stub: the whole query the
            // student writes is the submission, so the editor starts empty.
            setLanguage('sql')
            setSourceCode('')
          } else {
            // Loading a challenge resets the workspace to the default tab
            // and force-injects the student-visible stub of the teacher's
            // starter_code (the hidden runner below the marker is
            // stripped). The single starter_code field belongs to the
            // Python tab; other tabs fall back to the built-in templates.
            setLanguage('python')
            setSourceCode(
              data.starter_code ? visibleTemplate(data.starter_code) : CODE_TEMPLATES.python,
            )
          }
        }

        // Restore the student's own last attempt at this challenge, in
        // whichever language they last used, instead of leaving them to
        // re-type work "Voltar ao desafio" would otherwise have discarded.
        // 404 (never submitted) is the common case, not an error — the
        // starter_code default set above stays in place.
        try {
          const last = await getLastSubmission(id)
          if (active) {
            setLanguage(last.language)
            setSourceCode(last.source_code)
          }
        } catch {
          // No previous submission, or a transient fetch error — either
          // way, keep the starter_code default.
        }
      } catch (err) {
        if (active) {
          setError(err instanceof Error ? err.message : 'Falha ao carregar o desafio')
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
    getChallengeStats(id)
      .then(setStats)
      .catch(() => {})
  }, [id])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!id) {
      setError('Desafio não encontrado')
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
      setError(err instanceof Error ? err.message : 'Falha ao enviar a solução')
    } finally {
      setSubmitting(false)
    }
  }

  // Template shown on a given tab: the visible stub of starter_code owns the
  // Python tab when the teacher defined one; every other tab uses its
  // built-in stub.
  function templateFor(lang: SubmissionLanguage): string {
    if (lang === 'python' && challenge?.starter_code) {
      return visibleTemplate(challenge.starter_code)
    }
    return CODE_TEMPLATES[lang]
  }

  if (!id) {
    return <Navigate to="/" replace />
  }

  return (
    <main className="page-shell page-shell--workspace">
      <Link className="back-link" to="/">
        ← Voltar aos desafios
      </Link>

      {loading ? <p className="status-message">Carregando desafio...</p> : null}

      {error ? <p className="status-message status-error">{error}</p> : null}

      {challenge ? (
        <>
        <div className="workspace">
          <section className="workspace__brief">
            <p className="eyebrow">Desafio</p>
            <h1>{challenge.title}</h1>

            <div className="constraints">
              <span className={`difficulty difficulty--${challenge.difficulty}`}>
                {challenge.difficulty}
              </span>
              {challenge.solved ? (
                <span className="verdict verdict--accepted">CONCLUÍDO</span>
              ) : null}
              <span className="constraint">
                Tempo <strong>{challenge.time_limit_ms}ms</strong>
              </span>
              <span className="constraint">
                Memória <strong>{challenge.memory_limit_mb}MB</strong>
              </span>
              <span className="constraint">/{challenge.slug}</span>
            </div>

            <p className="muted">{challenge.description}</p>

            {challenge.notes ? <p className="notes-callout">{challenge.notes}</p> : null}

            {challenge.language === 'sql' && challenge.sql_schema ? (
              <>
                <h2 className="section-title">Schema SQL</h2>
                <pre className="sample__io">{challenge.sql_schema}</pre>
              </>
            ) : null}

            {challenge.sample_test_cases.length > 0 ? (
              <>
                <h2 className="section-title">Exemplos</h2>
                {challenge.sample_test_cases.map((tc, i) => (
                  <div className="sample" key={i}>
                    <div className="sample__label">Exemplo {i + 1}</div>
                    <div className="sample__io">
                      <div>
                        <span>{challenge.language === 'sql' ? 'Dados de Seed' : 'Entrada'}</span>
                        <pre>{tc.input || '(vazio)'}</pre>
                      </div>
                      <div>
                        <span>
                          {challenge.language === 'sql' ? 'Resultado Esperado' : 'Saída esperada'}
                        </span>
                        <pre>{tc.expected_output}</pre>
                      </div>
                    </div>
                  </div>
                ))}
              </>
            ) : null}
          </section>

          <form className="workspace__editor" onSubmit={handleSubmit}>
            <div className="editor-toolbar">
              {challenge.language === 'sql' ? (
                <span className="editor-toolbar__title">{FILE_NAMES.sql}</span>
              ) : (
                <>
                  <div className="editor-tabs" role="tablist" aria-label="Linguagem">
                    {LANGUAGES.map((lang) => (
                      <button
                        key={lang}
                        type="button"
                        role="tab"
                        aria-selected={language === lang}
                        className={
                          language === lang ? 'editor-tab editor-tab--active' : 'editor-tab'
                        }
                        disabled={submitting}
                        onClick={() => {
                          // Swap templates only while the editor still holds the
                          // untouched template of the active tab; user edits stay.
                          if (sourceCode === '' || sourceCode === templateFor(language)) {
                            setSourceCode(templateFor(lang))
                          }
                          setLanguage(lang)
                        }}
                      >
                        {FILE_NAMES[lang]}
                      </button>
                    ))}
                  </div>
                  <span className="editor-toolbar__title">Editor de código</span>
                </>
              )}
            </div>

            <div className="editor-host">
              <Editor
                height="100%"
                theme="vs-dark"
                language={language}
                value={sourceCode}
                onChange={(v) => setSourceCode(v ?? '')}
                options={{
                  fontFamily: 'JetBrains Mono, Consolas, monospace',
                  fontSize: 13,
                  minimap: { enabled: false },
                  automaticLayout: true,
                  padding: { top: 12 },
                  readOnly: submitting,
                }}
              />
            </div>

            <div className="editor-actions">
              <p className="editor-actions__hint">
                {challenge.language === 'sql'
                  ? 'Sua query roda em um sandbox isolado contra todos os casos de teste.'
                  : 'Sua solução roda em um sandbox isolado contra todos os casos de teste.'}
              </p>
              <button type="submit" className="challenge-submit" disabled={submitting}>
                {submitting ? 'Enviando...' : 'Enviar solução'}
              </button>
            </div>
          </form>
        </div>

          <section className="history-section">
            <h2 className="section-title">Métricas</h2>
            {stats !== null && stats.total_runs > 0 ? (
              <div className="panel metrics-panel">
                {stats.faster_than_pct !== null ? (
                  <p className="metrics-highlight">
                    Seu código foi mais rápido que <strong>{stats.faster_than_pct}%</strong> das
                    submissões para este desafio!
                  </p>
                ) : (
                  <p className="metrics-highlight metrics-highlight--muted">
                    Envie uma solução aceita para ver sua posição na distribuição.
                  </p>
                )}
                {(() => {
                  const maxCount = Math.max(...stats.buckets.map((b) => b.count), 1)
                  const best = stats.user_best_ms
                  const userBucketIdx =
                    best !== null ? stats.buckets.findIndex((b) => best <= b.up_to_ms) : -1
                  const lastBucket = stats.buckets[stats.buckets.length - 1]
                  return (
                    <>
                      <div
                        className="histogram"
                        role="img"
                        aria-label={`Distribuição do tempo de execução de ${stats.total_runs} submissões aceitas`}
                      >
                        {stats.buckets.map((b, i) => (
                          <div
                            className="histogram__slot"
                            key={b.up_to_ms}
                            title={`≤ ${b.up_to_ms}ms — ${b.count} ${b.count === 1 ? 'submissão' : 'submissões'}`}
                          >
                            <div
                              className={
                                i === userBucketIdx
                                  ? 'histogram__bar histogram__bar--you'
                                  : 'histogram__bar'
                              }
                              style={{
                                height: `${b.count > 0 ? Math.max((b.count / maxCount) * 100, 6) : 2}%`,
                              }}
                            />
                            {i === userBucketIdx ? (
                              <span className="histogram__you">você</span>
                            ) : null}
                          </div>
                        ))}
                      </div>
                      <div className="histogram__axis">
                        <span>0ms</span>
                        <span>{lastBucket ? `${lastBucket.up_to_ms}ms` : ''}</span>
                      </div>
                      <p className="histogram__caption">
                        Tempo de execução — {stats.total_runs}{' '}
                        {stats.total_runs === 1 ? 'submissão aceita' : 'submissões aceitas'} na
                        plataforma
                      </p>
                    </>
                  )
                })()}
              </div>
            ) : (
              <p className="status-message">
                Ainda não há métricas de execução para este desafio — seja a primeira solução
                aceita.
              </p>
            )}
          </section>

          <section className="history-section">
            <h2 className="section-title">Submissões recentes</h2>
            <div className="datagrid">
              <table>
                <thead>
                  <tr>
                    <th>Veredito</th>
                    <th>Linguagem</th>
                    <th>Data</th>
                  </tr>
                </thead>
                <tbody>
                  {submissions.length > 0 ? (
                    submissions.map((sub) => (
                      <tr key={sub.id}>
                        <td>
                          <VerdictBadge status={sub.status} />
                          {isTeacher && sub.is_test_run ? (
                            <span className="verdict test-run-tag">TESTE</span>
                          ) : null}
                        </td>
                        <td>{sub.language}</td>
                        <td className="muted">{new Date(sub.created_at).toLocaleString('pt-BR')}</td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td colSpan={3} className="datagrid__empty">
                        Nenhuma submissão ainda — envie sua primeira solução.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </section>
        </>
      ) : null}
    </main>
  )
}
