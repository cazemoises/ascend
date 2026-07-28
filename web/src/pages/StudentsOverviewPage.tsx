import { useEffect, useState } from 'react'

import { getStudentsOverview, type StudentOverview } from '../api'
import { VerdictBadge } from '../components/VerdictBadge'

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('pt-BR')
}

export function StudentsOverviewPage() {
  const [students, setStudents] = useState<StudentOverview[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    getStudentsOverview()
      .then((data) => {
        if (active) {
          setStudents(data)
        }
      })
      .catch((err) => {
        if (active) {
          setError(err instanceof Error ? err.message : 'Falha ao carregar o progresso dos alunos')
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })
    return () => {
      active = false
    }
  }, [])

  return (
    <main className="page-shell">
      <section className="hero">
        <p className="eyebrow">Área do professor</p>
        <h1>Progresso</h1>
        <p className="muted">Todos os alunos que já interagiram com desafios ou listas.</p>
      </section>

      {loading ? <p className="status-message">Carregando progresso...</p> : null}
      {error ? <p className="status-message status-error">{error}</p> : null}

      {!loading && !error && students.length === 0 ? (
        <p className="status-message">Nenhum aluno interagiu com desafios ou listas ainda.</p>
      ) : null}

      <div className="student-roster">
        {students.map((student) => (
          <article key={student.student_id} className="panel student-card">
            <h2 className="student-card__email">{student.email}</h2>

            <div className="student-card__section">
              <h3 className="section-title">Desafios</h3>
              {student.challenges.length === 0 ? (
                <p className="muted student-card__empty">Nenhum desafio enviado ainda.</p>
              ) : (
                <ul className="student-card__list">
                  {student.challenges.map((c) => (
                    <li key={c.challenge_id} className="student-card__row">
                      <span>{c.challenge_title}</span>
                      <VerdictBadge status={c.best_verdict} />
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className="student-card__section">
              <h3 className="section-title">Listas</h3>
              {student.list_completions.length === 0 ? (
                <p className="muted student-card__empty">Nenhum item concluído ainda.</p>
              ) : (
                <ul className="student-card__list">
                  {student.list_completions.map((completion) => (
                    <li
                      key={`${completion.list_title}-${completion.item_title}-${completion.completed_at}`}
                      className="student-card__row"
                    >
                      <span>
                        {completion.list_title} — {completion.item_title}
                      </span>
                      <span className="muted">{formatDate(completion.completed_at)}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </article>
        ))}
      </div>
    </main>
  )
}
