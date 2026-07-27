import { useEffect, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import { useNavigate, useParams } from 'react-router-dom'
import rehypeHighlight from 'rehype-highlight'

import {
  createListItem,
  createProblemList,
  deleteListItem,
  getProblemList,
  importProblemList,
  updateListItem,
  updateProblemList,
  type ImportProblemListInput,
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

function toDateInputValue(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

// Monday–Sunday of the current week, in the user's local time — the common
// case a teacher opening "Nova lista" wants pre-filled, adjustable if not.
function currentWeekRange(): { start: string; end: string } {
  const now = new Date()
  const day = now.getDay()
  const diffToMonday = day === 0 ? -6 : 1 - day
  const monday = new Date(now)
  monday.setDate(now.getDate() + diffToMonday)
  const sunday = new Date(monday)
  sunday.setDate(monday.getDate() + 6)
  return { start: toDateInputValue(monday), end: toDateInputValue(sunday) }
}

// The API returns week_start/week_end as full timestamps at UTC midnight;
// slicing the date portion avoids a local-timezone parse shifting the day.
function isoToDateInputValue(iso: string | null): string {
  return iso ? iso.slice(0, 10) : ''
}

type JsonImportPreview = {
  title: string
  items: { title: string; difficulty: string }[]
}

// Reads only what the preview needs to display — the backend is the sole
// source of truth for shape/business validation (difficulty enum, date
// format, required fields).
function toImportPreview(raw: unknown): JsonImportPreview {
  const obj = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const itemsRaw = Array.isArray(obj.items) ? obj.items : []
  return {
    title: typeof obj.title === 'string' ? obj.title : '',
    items: itemsRaw.map((raw) => {
      const item = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
      return {
        title: typeof item.title === 'string' ? item.title : '',
        difficulty: typeof item.difficulty === 'string' ? item.difficulty : '',
      }
    }),
  }
}

export function ListFormPage() {
  const { id } = useParams<{ id?: string }>()
  const navigate = useNavigate()
  const editing = id !== undefined

  const [title, setTitle] = useState('')
  const [weekLabel, setWeekLabel] = useState('')
  // Nova lista: pre-fill the current week's Monday–Sunday range as the
  // initial render, not via an effect — the teacher can still adjust or
  // clear both fields before saving. Editing an existing list overwrites
  // these once its data loads below.
  const [weekStart, setWeekStart] = useState(() => (editing ? '' : currentWeekRange().start))
  const [weekEnd, setWeekEnd] = useState(() => (editing ? '' : currentWeekRange().end))
  const [description, setDescription] = useState('')
  const [published, setPublished] = useState(false)
  const [items, setItems] = useState<ItemDraft[]>([{ ...EMPTY_ITEM }])
  const [originalItemIds, setOriginalItemIds] = useState<string[]>([])

  const [loading, setLoading] = useState(editing)
  const [error, setError] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  // Import mode only exists on "Nova lista" — editing an existing list keeps
  // the manual form.
  const [mode, setMode] = useState<'manual' | 'json'>('manual')
  const [jsonText, setJsonText] = useState('')
  const [jsonError, setJsonError] = useState<string | null>(null)
  const [jsonPreview, setJsonPreview] = useState<JsonImportPreview | null>(null)

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
        setWeekStart(isoToDateInputValue(detail.week_start))
        setWeekEnd(isoToDateInputValue(detail.week_end))
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

  function handlePreviewJson() {
    setJsonError(null)
    try {
      const parsed: unknown = JSON.parse(jsonText)
      setJsonPreview(toImportPreview(parsed))
    } catch (err) {
      setJsonPreview(null)
      setJsonError(err instanceof Error ? err.message : 'JSON inválido')
    }
  }

  async function handleImportSubmit() {
    let parsed: unknown
    try {
      parsed = JSON.parse(jsonText)
    } catch (err) {
      setJsonError(err instanceof Error ? err.message : 'JSON inválido')
      return
    }
    setJsonError(null)

    setSaving(true)
    try {
      const created = await importProblemList(parsed as ImportProblemListInput)
      navigate(`/listas/${created.id}`)
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Falha ao importar a lista')
    } finally {
      setSaving(false)
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaveError(null)

    if (mode === 'json') {
      await handleImportSubmit()
      return
    }

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
        week_start: weekStart.trim() === '' ? null : weekStart,
        week_end: weekEnd.trim() === '' ? null : weekEnd,
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
          week_start: listPayload.week_start,
          week_end: listPayload.week_end,
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

        {!editing ? (
          <div className="studio__mode-tabs" role="tablist" aria-label="Modo de criação">
            <button
              type="button"
              role="tab"
              aria-selected={mode === 'manual'}
              className={mode === 'manual' ? 'studio__mode-tab studio__mode-tab--active' : 'studio__mode-tab'}
              onClick={() => setMode('manual')}
              disabled={saving}
            >
              Preencher manualmente
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === 'json'}
              className={mode === 'json' ? 'studio__mode-tab studio__mode-tab--active' : 'studio__mode-tab'}
              onClick={() => setMode('json')}
              disabled={saving}
            >
              Importar JSON
            </button>
          </div>
        ) : null}

        {mode === 'json' ? (
          <section className="studio__panel">
            <h2 className="studio__panel-title">Importar JSON</h2>
            <p className="studio__panel-hint">
              Cole o JSON da lista completa (título, semana, descrição e itens) para criar tudo de
              uma vez.
            </p>
            <label className="studio__field--full">
              JSON
              <textarea
                className="input-code input-code--tall"
                value={jsonText}
                onChange={(e) => {
                  setJsonText(e.target.value)
                  setJsonPreview(null)
                  setJsonError(null)
                }}
                placeholder='{"title": "Bancos de dados", "items": [...]}'
                disabled={saving}
              />
            </label>
            {jsonError ? <p className="status-message status-error">{jsonError}</p> : null}
            <button
              type="button"
              className="btn-secondary"
              onClick={handlePreviewJson}
              disabled={saving || jsonText.trim() === ''}
            >
              Pré-visualizar
            </button>
            {jsonPreview ? (
              <div className="studio__json-preview">
                <p className="studio__panel-hint">
                  <strong>{jsonPreview.title || '(sem título)'}</strong> — {jsonPreview.items.length}{' '}
                  item(ns)
                </p>
                <ul>
                  {jsonPreview.items.map((it, i) => (
                    <li key={i}>
                      {it.title || '(sem título)'} — {it.difficulty || '(sem dificuldade)'}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </section>
        ) : (
        <>
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
            <label className="studio__field--3">
              Início da semana (opcional)
              <input
                type="date"
                value={weekStart}
                onChange={(e) => setWeekStart(e.target.value)}
                disabled={saving}
              />
            </label>
            <label className="studio__field--3">
              Fim da semana (opcional)
              <input
                type="date"
                value={weekEnd}
                onChange={(e) => setWeekEnd(e.target.value)}
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
              <div className="studio__field--full list-detail__description markdown-content">
                <span className="studio__panel-hint">Prévia:</span>
                <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{description}</ReactMarkdown>
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
                  <div className="list-detail__description markdown-content">
                    <span className="studio__panel-hint">Prévia:</span>
                    <ReactMarkdown rehypePlugins={[rehypeHighlight]}>{item.body}</ReactMarkdown>
                  </div>
                ) : null}
              </div>
            ))}
          </div>

          <button type="button" className="btn-add" onClick={addItem} disabled={saving}>
            + Adicionar item
          </button>
        </section>
        </>
        )}
      </form>
    </main>
  )
}
