import Editor from '@monaco-editor/react'
import { useEffect, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import { Link, Navigate, useNavigate, useParams } from 'react-router-dom'
import rehypeHighlight from 'rehype-highlight'

import {
  createSubmission,
  DIFFICULTY_LABELS,
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
import { TelemetryChip } from '../components/TelemetryChip'
import { VerdictBadge } from '../components/VerdictBadge'
import { defineAscendMonacoTheme } from '../lib/monacoTheme'

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
  // Plain and complete, like the go/javascript templates above — not using
  // the [[ASCEND::RUNNER]] marker split. That convention is scoped to a
  // challenge's own custom starter_code (Python tab only, see templateFor
  // below), not these generic per-language fallbacks; go/javascript don't
  // use it here either, so this stays consistent with them rather than
  // inventing a new split just for Java's default template. The judge
  // compiles this as Solution.java and runs `java Solution`, so the
  // runnable class must keep this exact name.
  java: `import java.util.Scanner;

public class Solution {
    public static void main(String[] args) {
        Scanner scanner = new Scanner(System.in);
        int a = scanner.nextInt();
        int b = scanner.nextInt();
        // TODO
        System.out.println();
    }
}
`,
  c: `#include <stdio.h>

int main(void) {
    int a, b;
    scanf("%d %d", &a, &b);
    // TODO
    printf("\\n");
    return 0;
}
`,
  cpp: `#include <iostream>

int main() {
    int a, b;
    std::cin >> a >> b;
    // TODO
    std::cout << std::endl;
    return 0;
}
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
const LANGUAGES: SubmissionLanguage[] = ['python', 'go', 'javascript', 'java', 'c', 'cpp']

const FILE_NAMES: Record<SubmissionLanguage, string> = {
  sql: 'query.sql',
  python: 'solution.py',
  go: 'main.go',
  javascript: 'solution.js',
  java: 'Solution.java',
  c: 'solution.c',
  cpp: 'solution.cpp',
}

export function ChallengePage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { isTeacher } = useAuth()

  const [challenge, setChallenge] = useState<Challenge | null>(null)
  const [language, setLanguage] = useState<SubmissionLanguage>('python')
  // Keyed by language so each tab keeps its own edits independently — a
  // single shared string here previously let an edit made under one tab
  // leak into another tab's syntax highlighting when switching (the editor
  // only swapped in the new tab's template while the old one was still
  // untouched, so any edit anywhere else stuck around under the new
  // language). Falls back to templateFor(language) below for tabs that
  // haven't been touched yet.
  const [codeByLanguage, setCodeByLanguage] = useState<Partial<Record<SubmissionLanguage, string>>>({})
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState<boolean>(false)
  const [submissions, setSubmissions] = useState<SubmissionSummary[]>([])
  const [stats, setStats] = useState<ChallengeStats | null>(null)
  // Métricas/Submissões live in a collapsible panel below the editor instead
  // of stacking as full sections beneath the workspace — keeps the page
  // fittable in the viewport (see .workspace/.page-shell--workspace).
  const [insightsOpen, setInsightsOpen] = useState<boolean>(false)
  const [insightsTab, setInsightsTab] = useState<'metrics' | 'submissions'>('metrics')

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
          // Loading a challenge resets the workspace to the default
          // language and clears every language's stored code — templateFor
          // derives each one's starting content (the teacher's starter_code
          // for Python, '' for SQL, built-in stubs otherwise) once
          // `challenge` is set above. A pinned challenge (data.language set
          // to anything) has no real choice, so it starts — and stays —
          // on that one language; only a true multi-language challenge
          // defaults to python.
          setCodeByLanguage({})
          setLanguage(data.language ?? 'python')
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
            setCodeByLanguage((prev) => ({ ...prev, [last.language]: last.source_code }))
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

  const sourceCode = codeByLanguage[language] ?? templateFor(language)

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
                {DIFFICULTY_LABELS[challenge.difficulty]}
              </span>
              {challenge.solved ? (
                <span className="verdict verdict--accepted">✓ Concluído</span>
              ) : null}
              <TelemetryChip
                timeLimitMs={challenge.time_limit_ms}
                memoryLimitMb={challenge.memory_limit_mb}
              />
            </div>

            <div className="muted markdown-content">
              <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{challenge.description}</ReactMarkdown>
            </div>

            {challenge.notes ? (
              <div className="notes-callout markdown-content">
                <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{challenge.notes}</ReactMarkdown>
              </div>
            ) : null}

            {challenge.language === 'sql' && challenge.sql_schema ? (
              <>
                <h2 className="section-title">Schema SQL</h2>
                <pre className="schema-sql">{challenge.sql_schema}</pre>
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
              {challenge.language ? (
                // Pinned to one language (any SubmissionLanguage, not just
                // 'sql' — see Challenge.language's comment in api.ts) —
                // there's no real choice to present, so a fixed filename
                // reads better than a single-option menu. This is the same
                // treatment 'sql' already got before pinning generalized
                // beyond it.
                <span className="editor-toolbar__title">{FILE_NAMES[challenge.language]}</span>
              ) : (
                <>
                  <select
                    className="editor-language-select"
                    aria-label="Linguagem"
                    value={language}
                    disabled={submitting}
                    onChange={(e) => setLanguage(e.target.value as SubmissionLanguage)}
                  >
                    {LANGUAGES.map((lang) => (
                      <option key={lang} value={lang}>
                        {FILE_NAMES[lang]}
                      </option>
                    ))}
                  </select>
                  <span className="editor-toolbar__title">Editor de código</span>
                </>
              )}
            </div>

            <div className="editor-host">
              <Editor
                height="100%"
                theme="ascend"
                beforeMount={defineAscendMonacoTheme}
                language={language}
                value={sourceCode}
                onChange={(v) => setCodeByLanguage((prev) => ({ ...prev, [language]: v ?? '' }))}
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

            <div className="workspace__insights">
              <button
                type="button"
                className="workspace__insights-toggle"
                aria-expanded={insightsOpen}
                onClick={() => setInsightsOpen((prev) => !prev)}
              >
                <span
                  className={insightsOpen ? 'audit-caret audit-caret--open' : 'audit-caret'}
                  aria-hidden="true"
                >
                  ▸
                </span>
                Métricas e submissões recentes
              </button>

              {insightsOpen ? (
                <div className="workspace__insights-body">
                  <div className="studio__mode-tabs" role="tablist" aria-label="Painel de informações">
                    <button
                      type="button"
                      role="tab"
                      aria-selected={insightsTab === 'metrics'}
                      className={
                        insightsTab === 'metrics'
                          ? 'studio__mode-tab studio__mode-tab--active'
                          : 'studio__mode-tab'
                      }
                      onClick={() => setInsightsTab('metrics')}
                    >
                      Métricas
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={insightsTab === 'submissions'}
                      className={
                        insightsTab === 'submissions'
                          ? 'studio__mode-tab studio__mode-tab--active'
                          : 'studio__mode-tab'
                      }
                      onClick={() => setInsightsTab('submissions')}
                    >
                      Submissões recentes
                    </button>
                  </div>

                  <div className="workspace__insights-panel">
                    {insightsTab === 'metrics' ? (
                      stats !== null && stats.total_runs > 0 ? (
                        <div className="panel metrics-panel">
                          {stats.faster_than_pct !== null ? (
                            <p className="metrics-highlight">
                              Seu código foi mais rápido que{' '}
                              <strong>{stats.faster_than_pct}%</strong> das submissões para este
                              desafio!
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
                              best !== null
                                ? stats.buckets.findIndex((b) => best <= b.up_to_ms)
                                : -1
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
                                  {stats.total_runs === 1
                                    ? 'submissão aceita'
                                    : 'submissões aceitas'}{' '}
                                  na plataforma
                                </p>
                              </>
                            )
                          })()}
                        </div>
                      ) : (
                        <p className="status-message">
                          Ainda não há métricas de execução para este desafio — seja a primeira
                          solução aceita.
                        </p>
                      )
                    ) : (
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
                                  <td className="muted">
                                    {new Date(sub.created_at).toLocaleString('pt-BR')}
                                  </td>
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
                    )}
                  </div>
                </div>
              ) : null}
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
        </>
      ) : null}
    </main>
  )
}
