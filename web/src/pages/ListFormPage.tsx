import { useEffect, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import { useNavigate, useParams } from 'react-router-dom'

import {
  createListItem,
  createProblemList,
  deleteListItem,
  getProblemList,
  updateListItem,
  updateProblemList,
  type ListItemDifficulty,
} from '../api'

type ItemDraft = {
  // Present only for items that already exist on the server — its absence
  // is what decides create vs. update on save.
  id?: string
  title: string
  difficulty: ListItemDifficulty
  is_bonus: boolean
  body: string
}

const EMPTY_ITEM: ItemDraft = {
  title: '',
  difficulty: 'easy',
  is_bonus: false,
  body: '',
}

const DIFFICULTY_OPTIONS: { value: ListItemDifficulty; label: string }[] = [
  { value: 'easy', label: 'Fácil' },
  { value: 'medium', label: 'Médio' },
  { value: 'hard', label: 'Difícil' },
]

export function ListFormPage() {
  const { id } = useParams<{ id?: string }>()
  const navigate = useNavigate()
  const editing = id !== undefined

  const [title, setTitle] = useState('')
  const [weekLabel, setWeekLabel] = useState('')
  const [description, setDescription] = useState('')
  const [published, setPublished] = useState(false)
  const [items, setItems] = useState<ItemDraft[]>([{ ...EMPTY_ITEM }])
  const [originalItemIds, setOriginalItemIds] = useState<string[]>([])

  const [loading, setLoading] = useState(editing)
  const [error, setError] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!editing || !id) {
      return
    }
    let active = true

    async function load() {
      try {
        setLoading(true)
        setError(null)
        const detail = await getProblemList(id as string)
        if (!active) {
          return
        }
        setTitle(detail.title)
        setWeekLabel(detail.week_label ?? '')
        setDescription(detail.description ?? '')
        setPublished(detail.published)
        setItems(
          detail.items.length > 0
            ? detail.items.map((it) => ({
                id: it.id,
                title: it.title,
                difficulty: it.difficulty,
                is_bonus: it.is_bonus,
                body: it.body,
              }))
            : [{ ...EMPTY_ITEM }],
        )
        setOriginalItemIds(detail.items.map((it) => it.id))
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

    void load()

    return () => {
      active = false
    }
  }, [editing, id])

  function updateItem(index: number, patch: Partial<ItemDraft>) {
    setItems((prev) => prev.map((it, i) => (i === index ? { ...it, ...patch } : it)))
  }

  function addItem() {
    setItems((prev) => [...prev, { ...EMPTY_ITEM }])
  }

  function removeItem(index: number) {
    setItems((prev) => prev.filter((_, i) => i !== index))
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaveError(null)

    // Blank rows are ignored; a partially-filled row is a teacher mistake
    // the backend would reject anyway — fail early with a clear message.
    const validItems = items.filter((it) => it.title.trim() !== '' || it.body.trim() !== '')
    const invalid = validItems.findIndex((it) => it.title.trim() === '' || it.body.trim() === '')
    if (invalid !== -1) {
      setSaveError(`Item ${invalid + 1}: título e conteúdo são obrigatórios.`)
      return
    }

    setSaving(true)
    try {
      const listPayload = {
        title,
        week_label: weekLabel.trim() === '' ? null : weekLabel,
        description: description.trim() === '' ? null : description,
        published,
      }

      let listId: string
      if (editing && id) {
        const updated = await updateProblemList(id, listPayload)
        listId = updated.id
      } else {
        const created = await createProblemList({
          title: listPayload.title,
          week_label: listPayload.week_label,
          description: listPayload.description,
        })
        listId = created.id
      }

      // Diff against the loaded set instead of delete-all-and-reinsert:
      // list items can carry student completions, and recreating them would
      // cascade-delete that progress on every edit.
      const keptIds = new Set<string>()
      for (const it of validItems) {
        const payload = {
          title: it.title,
          difficulty: it.difficulty,
          is_bonus: it.is_bonus,
          body: it.body,
        }
        if (it.id) {
          keptIds.add(it.id)
          await updateListItem(it.id, payload)
        } else {
          await createListItem(listId, payload)
        }
      }
      for (const oldId of originalItemIds) {
        if (!keptIds.has(oldId)) {
          await deleteListItem(oldId)
        }
      }

      navigate(`/listas/${listId}`)
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Falha ao salvar a lista')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <main className="page-shell studio">
        <p className="status-message">Carregando lista...</p>
      </main>
    )
  }

  if (error) {
    return (
      <main className="page-shell studio">
        <p className="status-message status-error">{error}</p>
      </main>
    )
  }

  return (
    <main className="page-shell studio">
      <form onSubmit={handleSubmit}>
        <header className="studio__header">
          <nav className="studio__breadcrumbs" aria-label="Breadcrumb">
            <span>Listas semanais</span>
            <span className="studio__breadcrumb-sep">/</span>
            <strong>{editing ? 'Editar lista' : 'Nova lista'}</strong>
          </nav>
          <div className="studio__actions">
            <button
              type="button"
              className="btn-secondary btn-ghost"
              onClick={() => navigate(-1)}
              disabled={saving}
            >
              Cancelar
            </button>
            <button type="submit" className="challenge-submit" disabled={saving}>
              {saving ? 'Salvando...' : editing ? 'Salvar alterações' : 'Criar lista'}
            </button>
          </div>
        </header>

        {saveError ? <p className="status-message status-error">{saveError}</p> : null}

        <section className="studio__panel">
          <h2 className="studio__panel-title">Identidade da lista</h2>
          <p className="studio__panel-hint">
            Título, período e a introdução em markdown que aparece no topo da lista.
          </p>
          <div className="studio__grid">
            <label className="studio__field--3">
              Título
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Recursão"
                required
                disabled={saving}
              />
            </label>
            <label className="studio__field--3">
              Semana (opcional)
              <input
                value={weekLabel}
                onChange={(e) => setWeekLabel(e.target.value)}
                placeholder="Semana 12"
                disabled={saving}
              />
            </label>
            <label className="studio__field--full">
              Descrição (markdown, opcional)
              <textarea
                className="input-code"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Introdução da lista..."
                disabled={saving}
              />
            </label>
            {description.trim() !== '' ? (
              <div className="studio__field--full list-detail__description">
                <span className="studio__panel-hint">Prévia:</span>
                <ReactMarkdown>{description}</ReactMarkdown>
              </div>
            ) : null}
            {editing ? (
              <label className="studio__field--full tc-visibility">
                <input
                  type="checkbox"
                  checked={published}
                  onChange={(e) => setPublished(e.target.checked)}
                  disabled={saving}
                />
                <span className="tc-visibility__track" aria-hidden="true" />
                <span className="tc-visibility__text">Publicada (visível para estudantes)</span>
              </label>
            ) : null}
          </div>
        </section>

        <section className="studio__panel">
          <h2 className="studio__panel-title">Itens da lista</h2>
          <p className="studio__panel-hint">
            Cada item é um exercício sem correção automática — regra, assinaturas e integração vão
            no corpo em markdown.
          </p>

          <div className="tc-list">
            {items.map((item, index) => (
              <div className="tc-card" key={index}>
                <div className="tc-card__head">
                  <span className="tc-card__label">Item {index + 1}</span>
                  <label className="tc-visibility">
                    <input
                      type="checkbox"
                      checked={item.is_bonus}
                      onChange={(e) => updateItem(index, { is_bonus: e.target.checked })}
                      disabled={saving}
                    />
                    <span className="tc-visibility__track" aria-hidden="true" />
                    <span className="tc-visibility__text">Bônus</span>
                  </label>
                  <button
                    type="button"
                    className="btn-remove"
                    onClick={() => removeItem(index)}
                    disabled={saving || items.length === 1}
                  >
                    Remover
                  </button>
                </div>
                <div className="tc-card__io">
                  <label>
                    Título
                    <input
                      value={item.title}
                      onChange={(e) => updateItem(index, { title: e.target.value })}
                      placeholder="Lista encadeada simples"
                      disabled={saving}
                    />
                  </label>
                  <label>
                    Dificuldade
                    <select
                      value={item.difficulty}
                      onChange={(e) =>
                        updateItem(index, { difficulty: e.target.value as ListItemDifficulty })
                      }
                      disabled={saving}
                    >
                      {DIFFICULTY_OPTIONS.map((opt) => (
                        <option key={opt.value} value={opt.value}>
                          {opt.label}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>
                <label>
                  Corpo (markdown)
                  <textarea
                    className="input-code input-code--tall"
                    value={item.body}
                    onChange={(e) => updateItem(index, { body: e.target.value })}
                    placeholder="Regra, assinaturas e como integrar com o código do aluno."
                    disabled={saving}
                  />
                </label>
                {item.body.trim() !== '' ? (
                  <div className="list-detail__description">
                    <span className="studio__panel-hint">Prévia:</span>
                    <ReactMarkdown>{item.body}</ReactMarkdown>
                  </div>
                ) : null}
              </div>
            ))}
          </div>

          <button type="button" className="btn-add" onClick={addItem} disabled={saving}>
            + Adicionar item
          </button>
        </section>
      </form>
    </main>
  )
}
