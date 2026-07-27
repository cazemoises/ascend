import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { listProblemLists, type ProblemList } from '../api'
import { useAuth } from '../auth/useAuth'
import { WeekChip } from '../components/WeekChip'

const MONTH_FULL_PT = [
  'JANEIRO', 'FEVEREIRO', 'MARÇO', 'ABRIL', 'MAIO', 'JUNHO',
  'JULHO', 'AGOSTO', 'SETEMBRO', 'OUTUBRO', 'NOVEMBRO', 'DEZEMBRO',
]

interface MonthGroup {
  key: string
  label: string
  lists: ProblemList[]
}

// Groups consecutive-by-key lists into month buckets, in the order they
// arrive — the backend already sorts week_start DESC NULLS LAST, so dated
// lists land newest-month-first and undated ones land in one trailing group.
function groupByMonth(lists: ProblemList[]): MonthGroup[] {
  const groups: MonthGroup[] = []
  const byKey = new Map<string, MonthGroup>()

  for (const list of lists) {
    let key = 'undated'
    let label = 'SEM DATA'
    if (list.week_start) {
      const d = new Date(list.week_start)
      key = `${d.getUTCFullYear()}-${d.getUTCMonth()}`
      label = `${MONTH_FULL_PT[d.getUTCMonth()]} ${d.getUTCFullYear()}`
    }

    let group = byKey.get(key)
    if (!group) {
      group = { key, label, lists: [] }
      byKey.set(key, group)
      groups.push(group)
    }
    group.lists.push(list)
  }

  return groups
}

// Shared row markup for every list card on this page — the current-week
// spotlight, the upcoming section, and the month-grouped history all render
// the same title/badge/meta/link body; only the spotlight gets the extra
// accent styling via `current`.
function ListCard({ list, current = false }: { list: ProblemList; current?: boolean }) {
  return (
    <article className={current ? 'challenge-row challenge-row--current' : 'challenge-row'}>
      <div className="challenge-row__body">
        <h2>
          {list.title}
          {!list.published ? <span className="verdict test-run-tag">RASCUNHO</span> : null}
        </h2>
        {list.week_label || (list.week_start && list.week_end) ? (
          <div className="challenge-row__meta">
            {list.week_label ? <p className="muted">{list.week_label}</p> : null}
            {list.week_start && list.week_end ? (
              <WeekChip weekStart={list.week_start} weekEnd={list.week_end} />
            ) : null}
          </div>
        ) : null}
      </div>
      <Link className="challenge-submit" to={`/listas/${list.id}`}>
        Ver lista
      </Link>
    </article>
  )
}

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

  const currentLists = lists.filter((list) => list.is_current)
  // Soonest-first — the opposite of the API's general DESC order, so this
  // subset gets its own sort rather than relying on the fetch order.
  const upcomingLists = [...lists]
    .filter((list) => list.is_upcoming)
    .sort((a, b) => (a.week_start ?? '').localeCompare(b.week_start ?? ''))
  const remainingLists = lists.filter((list) => !list.is_current && !list.is_upcoming)

  return (
    <main className="page-shell lists-page">
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
          <>
            {currentLists.length > 0 ? (
              <section className="list-current">
                <p className="list-current__label">Semana atual</p>
                <div className="challenge-feed">
                  {currentLists.map((list) => (
                    <ListCard key={list.id} list={list} current />
                  ))}
                </div>
              </section>
            ) : null}

            {upcomingLists.length > 0 ? (
              <section className="list-upcoming">
                <h2 className="list-section-header">Próximas</h2>
                <div className="challenge-feed">
                  {upcomingLists.map((list) => (
                    <ListCard key={list.id} list={list} />
                  ))}
                </div>
              </section>
            ) : null}

            {(currentLists.length > 0 || upcomingLists.length > 0) && remainingLists.length > 0 ? (
              <h2 className="list-section-header">Histórico</h2>
            ) : null}

            {remainingLists.length > 0
              ? (() => {
                  const groups = groupByMonth(remainingLists)
                  const showMonthHeaders = groups.length > 1
                  return groups.map((group) => (
                    <div key={group.key} className="list-month-group">
                      {showMonthHeaders ? (
                        <h2 className="list-month-header">{group.label}</h2>
                      ) : null}
                      <div className="challenge-feed">
                        {group.lists.map((list) => (
                          <ListCard key={list.id} list={list} />
                        ))}
                      </div>
                    </div>
                  ))
                })()
              : null}
          </>
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
