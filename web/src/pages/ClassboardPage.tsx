import { useEffect, useState } from 'react'
import { Navigate } from 'react-router-dom'

import { getTeacherScoreboard, type ClassScore } from '../api'
import { useAuth } from '../auth/useAuth'

export function ClassboardPage() {
  const { isTeacher } = useAuth()
  const [scores, setScores] = useState<ClassScore[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isTeacher) {
      return
    }
    let active = true
    getTeacherScoreboard()
      .then((data) => {
        if (active) {
          setScores(data)
        }
      })
      .catch((err) => {
        if (active) {
          setError(err instanceof Error ? err.message : 'Falha ao carregar as turmas')
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
  }, [isTeacher])

  if (!isTeacher) {
    return <Navigate to="/" replace />
  }

  const classes = new Map<string, { name: string; rows: ClassScore[] }>()
  for (const score of scores) {
    const entry = classes.get(score.class_id)
    if (entry !== undefined) {
      entry.rows.push(score)
    } else {
      classes.set(score.class_id, { name: score.class_name, rows: [score] })
    }
  }

  return (
    <main className="page-shell">
      <section className="hero">
        <p className="eyebrow">Área do professor</p>
        <h1>Turmas</h1>
        <p className="muted">
          Progresso dos estudantes por turma — desafios concluídos sobre o total publicado.
        </p>
      </section>

      {loading ? <p className="status-message">Carregando turmas...</p> : null}
      {error ? <p className="status-message status-error">{error}</p> : null}

      {!loading && !error && classes.size === 0 ? (
        <p className="status-message">
          Nenhuma turma com estudantes matriculados ainda. Crie turmas e matrículas para ver o
          quadro de progresso aqui.
        </p>
      ) : null}

      {[...classes.entries()].map(([classId, klass]) => (
        <section key={classId} className="classboard">
          <h2 className="section-title">{klass.name}</h2>
          <div className="datagrid">
            <table>
              <thead>
                <tr>
                  <th>Estudante</th>
                  <th>Concluídos</th>
                  <th>Progresso</th>
                </tr>
              </thead>
              <tbody>
                {klass.rows.map((row) => {
                  const pct = row.total > 0 ? Math.round((row.completed / row.total) * 100) : 0
                  return (
                    <tr key={row.student_id}>
                      <td>{row.student_email}</td>
                      <td className="muted">
                        {row.completed}/{row.total}
                      </td>
                      <td>
                        <div
                          className="scorebar"
                          role="img"
                          aria-label={`${pct}% dos desafios concluídos`}
                        >
                          <div className="scorebar__fill" style={{ width: `${pct}%` }} />
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </section>
      ))}
    </main>
  )
}
