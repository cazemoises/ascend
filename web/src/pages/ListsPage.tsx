import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { listProblemLists, type ProblemList } from '../api'
import { useAuth } from '../auth/useAuth'

export function ListsPage() {
  const { isTeacher } = useAuth()
  const [lists, setLists] = useState<ProblemList[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true

    listProblemLists()
      .then((data) => {
        if (active) {
          setLists(data)
        }
      })
      .catch((err) => {
        if (active) {
          setError(err instanceof Error ? err.message : 'Falha ao carregar as listas')
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
        <p className="eyebrow">Ascend</p>
        <h1>Listas semanais</h1>
        <p className="muted">
          Conteúdo publicado pelo professor — sem correção automática, o progresso é
          autodeclarado.
        </p>
      </section>

      {isTeacher ? (
        <div className="action-bar">
          <span className="action-bar__label">Área do professor</span>
          <Link className="challenge-submit" to="/listas/nova">
            Nova lista
          </Link>
        </div>
      ) : null}

      {loading ? <p className="status-message">Carregando listas...</p> : null}
      {error ? <p className="status-message status-error">{error}</p> : null}

      {!loading && !error ? (
        lists.length > 0 ? (
          <div className="challenge-feed">
            {lists.map((list) => (
              <article key={list.id} className="challenge-row">
                <div className="challenge-row__body">
                  <h2>
                    {list.title}
                    {!list.published ? (
                      <span className="verdict test-run-tag">RASCUNHO</span>
                    ) : null}
                  </h2>
                  {list.week_label ? <p className="muted">{list.week_label}</p> : null}
                </div>
                <Link className="challenge-submit" to={`/listas/${list.id}`}>
                  Ver lista
                </Link>
              </article>
            ))}
          </div>
        ) : (
          <p className="status-message">
            {isTeacher
              ? 'Você ainda não publicou nenhuma lista.'
              : 'Nenhuma lista publicada ainda.'}
          </p>
        )
      ) : null}
    </main>
  )
}
