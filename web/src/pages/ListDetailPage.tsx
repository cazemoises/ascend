import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import { Link, Navigate, useNavigate, useParams } from 'react-router-dom'

import {
  completeListItem,
  deleteListItem,
  deleteProblemList,
  getProblemList,
  reorderListItems,
  uncompleteListItem,
  updateProblemList,
  type ListItem,
  type ProblemListDetail,
} from '../api'
import { useAuth } from '../auth/useAuth'

const DIFFICULTY_LABELS: Record<string, string> = {
  easy: 'FÁCIL',
  medium: 'MÉDIO',
  hard: 'DIFÍCIL',
}

export function ListDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { user, isTeacher } = useAuth()

  const [detail, setDetail] = useState<ProblemListDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [pendingItemId, setPendingItemId] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  useEffect(() => {
    let active = true

    async function loadDetail() {
      if (!id) {
        if (active) {
          setError('Lista não encontrada')
          setLoading(false)
        }
        return
      }
      try {
        setLoading(true)
        setError(null)
        const data = await getProblemList(id)
        if (active) {
          setDetail(data)
        }
      } catch (err) {
        if (active) {
          setError(err instanceof Error ? err.message : 'Falha ao carregar a lista')
        }
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    }

    void loadDetail()

    return () => {
      active = false
    }
  }, [id])

  if (!id) {
    return <Navigate to="/listas" replace />
  }

  const isOwner = isTeacher && detail !== null && user !== null && detail.teacher_id === user.id

  async function handlePublish() {
    if (!detail) return
    setActionError(null)
    try {
      const updated = await updateProblemList(detail.id, {
        title: detail.title,
        week_label: detail.week_label,
        description: detail.description,
        published: true,
      })
      setDetail((prev) => (prev ? { ...prev, ...updated } : prev))
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Falha ao publicar a lista')
    }
  }

  async function handleDeleteList() {
    if (!detail) return
    if (!window.confirm(`Excluir a lista "${detail.title}"? Essa ação não pode ser desfeita.`)) {
      return
    }
    setActionError(null)
    try {
      await deleteProblemList(detail.id)
      navigate('/listas')
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Falha ao excluir a lista')
    }
  }

  async function handleDeleteItem(item: ListItem) {
    if (!window.confirm(`Excluir o item "${item.title}"?`)) {
      return
    }
    setActionError(null)
    try {
      await deleteListItem(item.id)
      setDetail((prev) =>
        prev ? { ...prev, items: prev.items.filter((it) => it.id !== item.id) } : prev,
      )
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Falha ao excluir o item')
    }
  }

  async function handleMoveItem(index: number, direction: -1 | 1) {
    if (!detail) return
    const items = detail.items
    const targetIndex = index + direction
    if (targetIndex < 0 || targetIndex >= items.length) {
      return
    }
    const a = items[index]
    const b = items[targetIndex]
    setActionError(null)
    try {
      await reorderListItems(detail.id, [
        { item_id: a.id, ordinal: b.ordinal },
        { item_id: b.id, ordinal: a.ordinal },
      ])
      setDetail((prev) => {
        if (!prev) return prev
        const next = [...prev.items]
        next[index] = { ...b, ordinal: a.ordinal }
        next[targetIndex] = { ...a, ordinal: b.ordinal }
        return { ...prev, items: next }
      })
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Falha ao reordenar os itens')
    }
  }

  async function handleToggleComplete(item: ListItem) {
    const nextCompleted = !item.completed
    setPendingItemId(item.id)
    setActionError(null)
    // Optimistic update: flip locally right away, revert if the call fails.
    setDetail((prev) =>
      prev
        ? {
            ...prev,
            items: prev.items.map((it) => (it.id === item.id ? { ...it, completed: nextCompleted } : it)),
          }
        : prev,
    )
    try {
      if (nextCompleted) {
        await completeListItem(item.id)
      } else {
        await uncompleteListItem(item.id)
      }
    } catch (err) {
      setDetail((prev) =>
        prev
          ? {
              ...prev,
              items: prev.items.map((it) =>
                it.id === item.id ? { ...it, completed: item.completed } : it,
              ),
            }
          : prev,
      )
      setActionError(err instanceof Error ? err.message : 'Falha ao marcar conclusão')
    } finally {
      setPendingItemId(null)
    }
  }

  return (
    <main className="page-shell">
      <Link className="back-link" to="/listas">
        ← Voltar às listas
      </Link>

      {loading ? <p className="status-message">Carregando lista...</p> : null}
      {error ? <p className="status-message status-error">{error}</p> : null}

      {!loading && !error && detail ? (
        <>
          <section className="panel submission-panel">
            <p className="eyebrow">Lista semanal</p>
            <h1>{detail.title}</h1>

            <div className="list-detail__meta">
              {detail.week_label ? <span className="constraint">{detail.week_label}</span> : null}
              {!detail.published ? <span className="verdict test-run-tag">RASCUNHO</span> : null}
            </div>

            {detail.description ? (
              <div className="list-detail__description">
                <ReactMarkdown>{detail.description}</ReactMarkdown>
              </div>
            ) : null}

            {actionError ? <p className="status-message status-error">{actionError}</p> : null}

            {isOwner ? (
              <div className="list-detail__actions">
                {!detail.published ? (
                  <button type="button" className="challenge-submit" onClick={() => void handlePublish()}>
                    Publicar
                  </button>
                ) : null}
                <Link className="btn-secondary" to={`/listas/${detail.id}/editar`}>
                  Editar lista
                </Link>
                <button type="button" className="btn-secondary" onClick={() => void handleDeleteList()}>
                  Excluir lista
                </button>
              </div>
            ) : null}
          </section>

          <section className="history-section">
            <h2 className="section-title">Itens</h2>
            {detail.items.length === 0 ? (
              <p className="status-message">Esta lista ainda não tem itens.</p>
            ) : (
              <div className="list-items">
                {detail.items.map((item, index) => {
                  const isOpen = expandedId === item.id
                  return (
                    <article className="list-item" key={item.id}>
                      <div className="list-item__head">
                        {!isTeacher ? (
                          <button
                            type="button"
                            className={
                              item.completed
                                ? 'list-complete-checkbox list-complete-checkbox--checked'
                                : 'list-complete-checkbox'
                            }
                            aria-pressed={item.completed === true}
                            aria-label={item.completed ? 'Marcar como não concluído' : 'Marcar como concluído'}
                            disabled={pendingItemId === item.id}
                            onClick={() => void handleToggleComplete(item)}
                          >
                            ✓
                          </button>
                        ) : null}

                        <button
                          type="button"
                          className="audit-toggle"
                          aria-expanded={isOpen}
                          onClick={() => setExpandedId(isOpen ? null : item.id)}
                        >
                          <span
                            className={isOpen ? 'audit-caret audit-caret--open' : 'audit-caret'}
                            aria-hidden="true"
                          >
                            ▸
                          </span>
                          <span className="list-item__title">{item.title}</span>
                        </button>

                        <span className={`list-badge list-badge--${item.difficulty}`}>
                          {DIFFICULTY_LABELS[item.difficulty] ?? item.difficulty}
                        </span>
                        {item.is_bonus ? <span className="list-badge">BÔNUS</span> : null}

                        {isOwner ? (
                          <div className="list-item__actions">
                            <button
                              type="button"
                              className="list-item__icon-btn"
                              title="Mover para cima"
                              aria-label="Mover item para cima"
                              disabled={index === 0}
                              onClick={() => void handleMoveItem(index, -1)}
                            >
                              ↑
                            </button>
                            <button
                              type="button"
                              className="list-item__icon-btn"
                              title="Mover para baixo"
                              aria-label="Mover item para baixo"
                              disabled={index === detail.items.length - 1}
                              onClick={() => void handleMoveItem(index, 1)}
                            >
                              ↓
                            </button>
                            <button
                              type="button"
                              className="list-item__icon-btn"
                              title="Excluir item"
                              aria-label={`Excluir ${item.title}`}
                              onClick={() => void handleDeleteItem(item)}
                            >
                              ✕
                            </button>
                          </div>
                        ) : null}
                      </div>

                      {isOpen ? (
                        <div className="list-item__body">
                          <ReactMarkdown>{item.body}</ReactMarkdown>
                        </div>
                      ) : null}
                    </article>
                  )
                })}
              </div>
            )}
          </section>
        </>
      ) : null}
    </main>
  )
}
