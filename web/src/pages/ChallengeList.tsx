import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'

import {
  createChallenge,
  createChallengeCollection,
  createChallengeGroup,
  importChallenges,
  listChallengeCollections,
  listChallengeGroups,
  listChallenges,
  listTestCases,
  replaceTestCases,
  reorderChallengeCollections,
  reorderChallengeGroups,
  updateChallenge,
  updateChallengeCollection,
  updateChallengeGroup,
  type Challenge,
  type ChallengeCollection,
  type ChallengeDifficulty,
  type ChallengeGroup,
  type ChallengeLanguage,
  type ImportChallengesInput,
} from '../api'
import { useAuth } from '../auth/useAuth'
import { TelemetryChip } from '../components/TelemetryChip'

type TestCaseDraft = {
  input: string
  expected_output: string
  is_sample: boolean
  order_matters: boolean
}

// '' means today's multi-language behavior (student picks a tab); 'sql'
// marks the challenge as SQL-only. The scope is deliberately just these two
// options — not a generic per-language restriction.
type LanguageDraft = '' | ChallengeLanguage

const EMPTY_DRAFT = {
  slug: '',
  title: '',
  description: '',
  difficulty: 'easy' as ChallengeDifficulty,
  time_limit_ms: '2000',
  memory_limit_mb: '256',
  starter_code: '',
  language: '' as LanguageDraft,
  sql_schema: '',
  collection_id: '',
}

const EMPTY_TEST_CASE: TestCaseDraft = {
  input: '',
  expected_output: '',
  is_sample: false,
  order_matters: false,
}

type JsonImportPreview = {
  collectionTitle: string
  challenges: { title: string; difficulty: string; language: string; testCaseCount: number }[]
}

// Reads only what the preview needs to display — the backend is the sole
// source of truth for shape/business validation.
function toImportPreview(raw: unknown): JsonImportPreview {
  const obj = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const challengesRaw = Array.isArray(obj.challenges) ? obj.challenges : []
  return {
    collectionTitle: typeof obj.collection_title === 'string' ? obj.collection_title : '',
    challenges: challengesRaw.map((raw) => {
      const ch = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
      const testCases = Array.isArray(ch.test_cases) ? ch.test_cases : []
      return {
        title: typeof ch.title === 'string' ? ch.title : '',
        difficulty: typeof ch.difficulty === 'string' ? ch.difficulty : '',
        language: typeof ch.language === 'string' ? ch.language : 'multi-linguagem',
        testCaseCount: testCases.length,
      }
    }),
  }
}

type CollectionBucket = {
  key: string
  label: string
  challenges: Challenge[]
}

// A challenge_group (level 1) containing its collections (level 2), or a
// bare collection sitting directly at level 1 because it has no group —
// no synthetic "no group" wrapper, per spec.
type TopLevelEntry =
  | { kind: 'group'; key: string; label: string; collections: CollectionBucket[] }
  | { kind: 'collection'; bucket: CollectionBucket }

// Groups consecutive-by-key challenges into a two-level group/collection
// structure, in the order they arrive — the backend already sorts by
// group.ordinal, then collection.ordinal within the group, then
// created_at, so this only detects boundaries, it doesn't re-sort (same
// rule the single-level version this replaced followed). Uncategorized
// challenges land in one trailing "SEM COLEÇÃO" bucket at the top level
// (a challenge with no collection can never have a group — group_id is
// only ever inherited through a collection).
function groupByGroupAndCollection(challenges: Challenge[]): TopLevelEntry[] {
  const entries: TopLevelEntry[] = []
  const groupsByKey = new Map<string, Extract<TopLevelEntry, { kind: 'group' }>>()
  const topCollectionsByKey = new Map<string, CollectionBucket>()

  for (const challenge of challenges) {
    const collectionKey = challenge.collection_id ?? 'uncategorized'
    const collectionLabel = challenge.collection_id ? (challenge.collection_title ?? '') : 'SEM COLEÇÃO'

    if (challenge.group_id) {
      let groupEntry = groupsByKey.get(challenge.group_id)
      if (!groupEntry) {
        groupEntry = {
          kind: 'group',
          key: challenge.group_id,
          label: challenge.group_title ?? '',
          collections: [],
        }
        groupsByKey.set(challenge.group_id, groupEntry)
        entries.push(groupEntry)
      }
      let bucket = groupEntry.collections.find((b) => b.key === collectionKey)
      if (!bucket) {
        bucket = { key: collectionKey, label: collectionLabel, challenges: [] }
        groupEntry.collections.push(bucket)
      }
      bucket.challenges.push(challenge)
    } else {
      let bucket = topCollectionsByKey.get(collectionKey)
      if (!bucket) {
        bucket = { key: collectionKey, label: collectionLabel, challenges: [] }
        topCollectionsByKey.set(collectionKey, bucket)
        entries.push({ kind: 'collection', bucket })
      }
      bucket.challenges.push(challenge)
    }
  }

  return entries
}

// Module-level (not defined inside ChallengeList) so it isn't recreated,
// and remounted, on every render.
function ChallengeCard({
  challenge,
  isTeacher,
  onEdit,
}: {
  challenge: Challenge
  isTeacher: boolean
  onEdit: (challenge: Challenge) => void
}) {
  return (
    <article className="challenge-row">
      <div className="challenge-row__badges">
        <span className={`difficulty difficulty--${challenge.difficulty}`}>
          {challenge.difficulty}
        </span>
        {challenge.solved ? <span className="verdict verdict--accepted">CONCLUÍDO</span> : null}
      </div>
      <div className="challenge-row__body">
        <h2>{challenge.title}</h2>
        <p>{challenge.description}</p>
        <div className="challenge-row__meta">
          <span className="challenge-row__slug">/{challenge.slug}</span>
          <TelemetryChip
            timeLimitMs={challenge.time_limit_ms}
            memoryLimitMb={challenge.memory_limit_mb}
          />
        </div>
      </div>
      <div className="challenge-row__actions">
        {isTeacher ? (
          <button
            type="button"
            className="challenge-row__edit"
            title="Editar desafio"
            aria-label={`Editar ${challenge.title}`}
            onClick={() => onEdit(challenge)}
          >
            <svg
              width="17"
              height="17"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
            </svg>
          </button>
        ) : null}
        <Link className="challenge-submit" to={`/challenges/${challenge.id}`}>
          Resolver desafio
        </Link>
      </div>
    </article>
  )
}

const STARTER_PLACEHOLDER = `def somar_numeros(a, b):
    # TODO: implemente aqui
    return 0

# [[ASCEND::RUNNER]]
import sys

def main():
    a, b = map(int, sys.stdin.read().split())
    print(somar_numeros(a, b))

if __name__ == '__main__':
    main()
`

export function ChallengeList() {
  const { isTeacher } = useAuth()
  const [challenges, setChallenges] = useState<Challenge[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [collections, setCollections] = useState<ChallengeCollection[]>([])
  // Keys of collapsed collection buckets (level 2, or a bare top-level
  // collection); a key's absence means expanded. Independent from
  // collapsedTopGroups below — collapsing a challenge_group (level 1)
  // doesn't affect the collapse state of the collections nested inside it.
  // Starting empty means every group is expanded by default. Resets on
  // reload — not persisted, per explicit scope.
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set())
  // Keys of collapsed challenge_groups (level 1).
  const [collapsedTopGroups, setCollapsedTopGroups] = useState<Set<string>>(new Set())

  const [creating, setCreating] = useState(false)
  const [editingChallengeId, setEditingChallengeId] = useState<string | null>(null)
  const [draft, setDraft] = useState(EMPTY_DRAFT)
  const [testCases, setTestCases] = useState<TestCaseDraft[]>([{ ...EMPTY_TEST_CASE }])
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  // Import mode only exists when creating — editing an existing challenge
  // keeps the manual form, same rule as ListFormPage.
  const [mode, setMode] = useState<'manual' | 'json'>('manual')
  const [jsonText, setJsonText] = useState('')
  const [jsonError, setJsonError] = useState<string | null>(null)
  const [jsonPreview, setJsonPreview] = useState<JsonImportPreview | null>(null)

  // Tracks whether the component is still mounted, for refreshChallenges'
  // post-mutation calls below (fired from event handlers, not an effect —
  // the mount fetch below guards itself the same way local effects
  // elsewhere in this file do, with its own `active` flag).
  const mountedRef = useRef(true)
  useEffect(() => {
    return () => {
      mountedRef.current = false
    }
  }, [])

  // Re-fetches the canonical, server-sorted-and-joined listing (collection
  // ordinal, then created_at, with collection_title already resolved).
  // Called after any mutation that can affect a challenge's collection
  // (create/update/import) instead of splicing the mutation's own response
  // into state — that response never carries collection_title (it isn't a
  // real column, only ListChallengesForViewer's join produces it), and a
  // naive prepend doesn't land the item in its collection's position
  // either. Re-fetching sidesteps both problems at once.
  async function refreshChallenges() {
    try {
      setLoading(true)
      setError(null)
      const data = await listChallenges()
      if (mountedRef.current) setChallenges(data)
    } catch (err) {
      if (mountedRef.current) {
        setError(err instanceof Error ? err.message : 'Falha ao carregar desafios')
      }
    } finally {
      if (mountedRef.current) setLoading(false)
    }
  }

  useEffect(() => {
    let active = true

    async function loadChallenges() {
      try {
        setLoading(true)
        setError(null)
        const data = await listChallenges()
        if (active) {
          setChallenges(data)
        }
      } catch (err) {
        if (active) {
          setError(err instanceof Error ? err.message : 'Falha ao carregar desafios')
        }
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    }

    void loadChallenges()

    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    if (!isTeacher) return
    let active = true
    listChallengeCollections()
      .then((data) => {
        if (active) setCollections(data)
      })
      .catch(() => {})
    return () => {
      active = false
    }
  }, [isTeacher])

  const [groups, setGroups] = useState<ChallengeGroup[]>([])

  useEffect(() => {
    if (!isTeacher) return
    let active = true
    listChallengeGroups()
      .then((data) => {
        if (active) setGroups(data)
      })
      .catch(() => {})
    return () => {
      active = false
    }
  }, [isTeacher])

  const [managingCollections, setManagingCollections] = useState(false)
  const [newCollectionTitle, setNewCollectionTitle] = useState('')
  const [newCollectionGroupId, setNewCollectionGroupId] = useState('')
  // Per-row draft title, keyed by collection id — only entries the teacher
  // has actually touched exist here; everything else falls back to the
  // collection's own title.
  const [collectionEdits, setCollectionEdits] = useState<Record<string, string>>({})
  const [collectionActionError, setCollectionActionError] = useState<string | null>(null)
  const [collectionSaving, setCollectionSaving] = useState(false)

  const [newGroupTitle, setNewGroupTitle] = useState('')
  const [groupEdits, setGroupEdits] = useState<Record<string, string>>({})

  async function handleCreateCollection() {
    const title = newCollectionTitle.trim()
    if (title === '') return
    setCollectionSaving(true)
    setCollectionActionError(null)
    try {
      const created = await createChallengeCollection({
        title,
        group_id: newCollectionGroupId === '' ? null : newCollectionGroupId,
      })
      setCollections((prev) => [...prev, created])
      setNewCollectionTitle('')
      setNewCollectionGroupId('')
    } catch (err) {
      setCollectionActionError(err instanceof Error ? err.message : 'Falha ao criar coleção')
    } finally {
      setCollectionSaving(false)
    }
  }

  async function handleRenameCollection(cc: ChallengeCollection) {
    const title = (collectionEdits[cc.id] ?? cc.title).trim()
    if (title === '' || title === cc.title) return
    setCollectionSaving(true)
    setCollectionActionError(null)
    try {
      const updated = await updateChallengeCollection(cc.id, {
        title,
        ordinal: cc.ordinal,
        group_id: cc.group_id,
      })
      setCollections((prev) => prev.map((c) => (c.id === cc.id ? updated : c)))
      await refreshChallenges()
    } catch (err) {
      setCollectionActionError(err instanceof Error ? err.message : 'Falha ao renomear coleção')
    } finally {
      setCollectionSaving(false)
    }
  }

  async function handleChangeCollectionGroup(cc: ChallengeCollection, groupId: string) {
    setCollectionSaving(true)
    setCollectionActionError(null)
    try {
      const updated = await updateChallengeCollection(cc.id, {
        title: cc.title,
        ordinal: cc.ordinal,
        group_id: groupId === '' ? null : groupId,
      })
      setCollections((prev) => prev.map((c) => (c.id === cc.id ? updated : c)))
      await refreshChallenges()
    } catch (err) {
      setCollectionActionError(err instanceof Error ? err.message : 'Falha ao mudar o grupo da coleção')
    } finally {
      setCollectionSaving(false)
    }
  }

  async function handleMoveCollection(index: number, direction: -1 | 1) {
    const targetIndex = index + direction
    if (targetIndex < 0 || targetIndex >= collections.length) return
    const a = collections[index]
    const b = collections[targetIndex]
    setCollectionActionError(null)
    try {
      await reorderChallengeCollections([
        { id: a.id, ordinal: b.ordinal },
        { id: b.id, ordinal: a.ordinal },
      ])
      setCollections((prev) => {
        const next = [...prev]
        next[index] = { ...b, ordinal: a.ordinal }
        next[targetIndex] = { ...a, ordinal: b.ordinal }
        return next
      })
      await refreshChallenges()
    } catch (err) {
      setCollectionActionError(err instanceof Error ? err.message : 'Falha ao reordenar coleções')
    }
  }

  async function handleCreateGroup() {
    const title = newGroupTitle.trim()
    if (title === '') return
    setCollectionSaving(true)
    setCollectionActionError(null)
    try {
      const created = await createChallengeGroup({ title })
      setGroups((prev) => [...prev, created])
      setNewGroupTitle('')
    } catch (err) {
      setCollectionActionError(err instanceof Error ? err.message : 'Falha ao criar grupo')
    } finally {
      setCollectionSaving(false)
    }
  }

  async function handleRenameGroup(cg: ChallengeGroup) {
    const title = (groupEdits[cg.id] ?? cg.title).trim()
    if (title === '' || title === cg.title) return
    setCollectionSaving(true)
    setCollectionActionError(null)
    try {
      const updated = await updateChallengeGroup(cg.id, { title, ordinal: cg.ordinal })
      setGroups((prev) => prev.map((g) => (g.id === cg.id ? updated : g)))
    } catch (err) {
      setCollectionActionError(err instanceof Error ? err.message : 'Falha ao renomear grupo')
    } finally {
      setCollectionSaving(false)
    }
  }

  async function handleMoveGroup(index: number, direction: -1 | 1) {
    const targetIndex = index + direction
    if (targetIndex < 0 || targetIndex >= groups.length) return
    const a = groups[index]
    const b = groups[targetIndex]
    setCollectionActionError(null)
    try {
      await reorderChallengeGroups([
        { id: a.id, ordinal: b.ordinal },
        { id: b.id, ordinal: a.ordinal },
      ])
      setGroups((prev) => {
        const next = [...prev]
        next[index] = { ...b, ordinal: a.ordinal }
        next[targetIndex] = { ...a, ordinal: b.ordinal }
        return next
      })
      await refreshChallenges()
    } catch (err) {
      setCollectionActionError(err instanceof Error ? err.message : 'Falha ao reordenar grupos')
    }
  }

  function toggleGroupCollapsed(key: string) {
    setCollapsedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return next
    })
  }

  function toggleTopGroupCollapsed(key: string) {
    setCollapsedTopGroups((prev) => {
      const next = new Set(prev)
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return next
    })
  }

  function updateTestCase(index: number, patch: Partial<TestCaseDraft>) {
    setTestCases((prev) => prev.map((tc, i) => (i === index ? { ...tc, ...patch } : tc)))
  }

  function addTestCase() {
    setTestCases((prev) => [...prev, { ...EMPTY_TEST_CASE }])
  }

  function removeTestCase(index: number) {
    setTestCases((prev) => prev.filter((_, i) => i !== index))
  }

  function closeStudio() {
    setDraft(EMPTY_DRAFT)
    setTestCases([{ ...EMPTY_TEST_CASE }])
    setSaveError(null)
    setEditingChallengeId(null)
    setCreating(false)
    setMode('manual')
    setJsonText('')
    setJsonError(null)
    setJsonPreview(null)
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
      await importChallenges(parsed as ImportChallengesInput)
      await refreshChallenges()
      closeStudio()
    } catch (err) {
      // Backend error is shown verbatim — it already identifies which
      // challenge/test case and field failed.
      setSaveError(err instanceof Error ? err.message : 'Falha ao importar desafios')
    } finally {
      setSaving(false)
    }
  }

  async function openEditor(challenge: Challenge) {
    setError(null)
    setSaveError(null)
    try {
      const cases = await listTestCases(challenge.id)
      setDraft({
        slug: challenge.slug,
        title: challenge.title,
        description: challenge.description,
        difficulty: challenge.difficulty,
        time_limit_ms: String(challenge.time_limit_ms),
        memory_limit_mb: String(challenge.memory_limit_mb),
        starter_code: challenge.starter_code ?? '',
        language: challenge.language ?? '',
        sql_schema: challenge.sql_schema ?? '',
        collection_id: challenge.collection_id ?? '',
      })
      setTestCases(
        cases.length > 0
          ? cases.map((tc) => ({
              input: tc.input,
              expected_output: tc.expected_output,
              is_sample: tc.is_sample,
              order_matters: tc.order_matters,
            }))
          : [{ ...EMPTY_TEST_CASE }],
      )
      setEditingChallengeId(challenge.id)
      setCreating(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Falha ao carregar o desafio para edição')
    }
  }

  async function handlePublish(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSaveError(null)

    if (mode === 'json') {
      await handleImportSubmit()
      return
    }

    // Blank rows are ignored; a filled row without expected output is a
    // teacher mistake the backend would reject anyway — fail early.
    const suite = testCases.filter(
      (tc) => tc.input.trim() !== '' || tc.expected_output.trim() !== '',
    )
    const invalid = suite.findIndex((tc) => tc.expected_output.trim() === '')
    if (invalid !== -1) {
      setSaveError(`Caso de teste ${invalid + 1}: a saída esperada (stdout) é obrigatória.`)
      return
    }
    if (draft.language === 'sql' && draft.sql_schema.trim() === '') {
      setSaveError('Schema SQL é obrigatório para desafios SQL.')
      return
    }

    setSaving(true)
    try {
      const timeLimit = Number.parseInt(draft.time_limit_ms, 10)
      const memoryLimit = Number.parseInt(draft.memory_limit_mb, 10)
      const payload = {
        slug: draft.slug,
        title: draft.title,
        description: draft.description,
        difficulty: draft.difficulty,
        time_limit_ms: Number.isNaN(timeLimit) ? undefined : timeLimit,
        memory_limit_mb: Number.isNaN(memoryLimit) ? undefined : memoryLimit,
        starter_code: draft.starter_code.trim() === '' ? null : draft.starter_code,
        language: draft.language === '' ? null : draft.language,
        sql_schema: draft.language === 'sql' ? draft.sql_schema : null,
        collection_id: draft.collection_id === '' ? null : draft.collection_id,
      }
      const suiteInput = suite.map((tc) => ({
        input: tc.input,
        expected_output: tc.expected_output,
        is_sample: tc.is_sample,
        order_matters: tc.order_matters,
      }))

      if (editingChallengeId !== null) {
        await updateChallenge(editingChallengeId, payload)
        await replaceTestCases(editingChallengeId, suiteInput)
        await refreshChallenges()
        closeStudio()
        return
      }

      const created = await createChallenge(payload)
      try {
        await replaceTestCases(created.id, suiteInput)
      } catch (err) {
        await refreshChallenges()
        setSaveError(
          'O desafio foi publicado, mas a suíte de testes falhou ao salvar' +
            (err instanceof Error ? ` (${err.message})` : '') +
            '. Use o botão Editar do desafio para reenviá-la.',
        )
        return
      }

      await refreshChallenges()
      closeStudio()
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Falha ao salvar o desafio')
    } finally {
      setSaving(false)
    }
  }

  if (isTeacher && creating) {
    return (
      <main className="page-shell studio">
        <form onSubmit={handlePublish}>
          <header className="studio__header">
            <nav className="studio__breadcrumbs" aria-label="Breadcrumb">
              <span>Área do Professor</span>
              <span className="studio__breadcrumb-sep">/</span>
              <strong>{editingChallengeId !== null ? 'Editar Desafio' : 'Novo Desafio'}</strong>
            </nav>
            <div className="studio__actions">
              <button
                type="button"
                className="btn-secondary btn-ghost"
                onClick={closeStudio}
                disabled={saving}
              >
                Cancelar
              </button>
              <button type="submit" className="challenge-submit" disabled={saving}>
                {saving
                  ? 'Salvando...'
                  : editingChallengeId !== null
                    ? 'Salvar alterações'
                    : 'Publicar desafio'}
              </button>
            </div>
          </header>

          {saveError ? <p className="status-message status-error">{saveError}</p> : null}

          {editingChallengeId === null ? (
            <div className="studio__mode-tabs" role="tablist" aria-label="Modo de criação">
              <button
                type="button"
                role="tab"
                aria-selected={mode === 'manual'}
                className={
                  mode === 'manual' ? 'studio__mode-tab studio__mode-tab--active' : 'studio__mode-tab'
                }
                onClick={() => setMode('manual')}
                disabled={saving}
              >
                Preencher manualmente
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={mode === 'json'}
                className={
                  mode === 'json' ? 'studio__mode-tab studio__mode-tab--active' : 'studio__mode-tab'
                }
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
                Cole o JSON de um ou mais desafios (com suíte de testes completa) para criar tudo
                de uma vez. Um desafio inválido no array cancela a importação inteira — nada é
                salvo até que todos passem.
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
                  placeholder='{"challenges": [{"slug": "...", "title": "...", "difficulty": "easy", "test_cases": [...]}]}'
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
                    <strong>{jsonPreview.challenges.length}</strong> desafio(s)
                    {jsonPreview.collectionTitle ? (
                      <>
                        {' '}
                        — coleção: <strong>{jsonPreview.collectionTitle}</strong>
                      </>
                    ) : null}
                  </p>
                  <ul>
                    {jsonPreview.challenges.map((ch, i) => (
                      <li key={i}>
                        {ch.title || '(sem título)'} — {ch.difficulty || '(sem dificuldade)'} —{' '}
                        {ch.language} — {ch.testCaseCount} caso(s) de teste
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
            </section>
          ) : (
          <>
          <section className="studio__panel">
            <h2 className="studio__panel-title">Identidade &amp; Restrições</h2>
            <p className="studio__panel-hint">
              Como o desafio aparece no feed e os limites de execução do container.
            </p>
            <div className="studio__grid">
              <label className="studio__field--3">
                Título
                <input
                  value={draft.title}
                  onChange={(e) => setDraft({ ...draft, title: e.target.value })}
                  placeholder="Soma de dois números"
                  required
                  disabled={saving}
                />
              </label>
              <label className="studio__field--3">
                Slug
                <input
                  value={draft.slug}
                  onChange={(e) => setDraft({ ...draft, slug: e.target.value })}
                  placeholder="soma-de-dois"
                  required
                  disabled={saving}
                />
              </label>
              <label className="studio__field--2">
                Dificuldade
                <select
                  value={draft.difficulty}
                  onChange={(e) =>
                    setDraft({ ...draft, difficulty: e.target.value as ChallengeDifficulty })
                  }
                  disabled={saving}
                >
                  <option value="easy">easy</option>
                  <option value="medium">medium</option>
                  <option value="hard">hard</option>
                </select>
              </label>
              <label className="studio__field--2">
                Modalidade
                <select
                  value={draft.language}
                  onChange={(e) => setDraft({ ...draft, language: e.target.value as LanguageDraft })}
                  disabled={saving}
                >
                  <option value="">Multi-linguagem (Python/Go/JS)</option>
                  <option value="sql">SQL</option>
                </select>
              </label>
              <label className="studio__field--2">
                Coleção
                <select
                  value={draft.collection_id}
                  onChange={(e) => setDraft({ ...draft, collection_id: e.target.value })}
                  disabled={saving}
                >
                  <option value="">Sem coleção</option>
                  {collections.map((cc) => (
                    <option key={cc.id} value={cc.id}>
                      {cc.title}
                    </option>
                  ))}
                </select>
              </label>
              <label className="studio__field--2">
                Limite de Tempo (ms)
                <input
                  type="number"
                  min={100}
                  step={100}
                  value={draft.time_limit_ms}
                  onChange={(e) => setDraft({ ...draft, time_limit_ms: e.target.value })}
                  disabled={saving}
                />
              </label>
              <label className="studio__field--2">
                Limite de Memória (MB)
                <input
                  type="number"
                  min={16}
                  step={16}
                  value={draft.memory_limit_mb}
                  onChange={(e) => setDraft({ ...draft, memory_limit_mb: e.target.value })}
                  disabled={saving}
                />
              </label>
              <div className="studio__field--full studio__telemetry-preview">
                <span className="studio__panel-hint">Assim vai aparecer para o aluno:</span>
                <TelemetryChip
                  timeLimitMs={Number.parseInt(draft.time_limit_ms, 10) || 2000}
                  memoryLimitMb={Number.parseInt(draft.memory_limit_mb, 10) || 256}
                />
              </div>
              <label className="studio__field--full">
                Descrição
                <textarea
                  className="input-code"
                  value={draft.description}
                  onChange={(e) => setDraft({ ...draft, description: e.target.value })}
                  placeholder="Leia dois inteiros da entrada padrão e imprima a soma."
                  disabled={saving}
                />
              </label>
            </div>
          </section>

          <section className="studio__panel">
            <h2 className="studio__panel-title">Suíte de Testes da Sandbox</h2>
            <p className="studio__panel-hint">
              Casos públicos aparecem como exemplos para o estudante; casos ocultos avaliam a
              solução silenciosamente no judge.
            </p>

            <div className="tc-list">
              {testCases.map((tc, index) => (
                <div className="tc-card" key={index}>
                  <div className="tc-card__head">
                    <span className="tc-card__label">Caso de Teste {index + 1}</span>
                    <label className="tc-visibility">
                      <input
                        type="checkbox"
                        checked={tc.is_sample}
                        onChange={(e) => updateTestCase(index, { is_sample: e.target.checked })}
                        disabled={saving}
                      />
                      <span className="tc-visibility__track" aria-hidden="true" />
                      <span className="tc-visibility__text">Caso de Teste Público</span>
                    </label>
                    {draft.language === 'sql' ? (
                      <label className="tc-visibility">
                        <input
                          type="checkbox"
                          checked={tc.order_matters}
                          onChange={(e) =>
                            updateTestCase(index, { order_matters: e.target.checked })
                          }
                          disabled={saving}
                        />
                        <span className="tc-visibility__track" aria-hidden="true" />
                        <span className="tc-visibility__text">Ordem importa</span>
                      </label>
                    ) : null}
                    <button
                      type="button"
                      className="btn-remove"
                      onClick={() => removeTestCase(index)}
                      disabled={saving || testCases.length === 1}
                    >
                      Remover
                    </button>
                  </div>
                  <div className="tc-card__io">
                    <label>
                      {draft.language === 'sql' ? 'Dados de Seed (INSERTs)' : 'Entrada Padrão (stdin)'}
                      <textarea
                        className="input-code"
                        value={tc.input}
                        onChange={(e) => updateTestCase(index, { input: e.target.value })}
                        placeholder={
                          draft.language === 'sql'
                            ? "INSERT INTO students VALUES (1,'Ana',9.5);"
                            : '2 3'
                        }
                        disabled={saving}
                      />
                    </label>
                    <label>
                      {draft.language === 'sql'
                        ? 'Resultado Esperado (colunas separadas por |, uma linha por registro)'
                        : 'Saída Esperada (stdout)'}
                      <textarea
                        className="input-code"
                        value={tc.expected_output}
                        onChange={(e) =>
                          updateTestCase(index, { expected_output: e.target.value })
                        }
                        placeholder={draft.language === 'sql' ? 'Ana|9.5' : '5'}
                        disabled={saving}
                      />
                    </label>
                  </div>
                </div>
              ))}
            </div>

            <button type="button" className="btn-add" onClick={addTestCase} disabled={saving}>
              + Adicionar Caso de Teste
            </button>
          </section>

          {draft.language === 'sql' ? (
            <section className="studio__panel">
              <h2 className="studio__panel-title">Schema SQL</h2>
              <p className="studio__panel-hint">
                DDL e dados base compartilhados por todos os casos de teste (ex:{' '}
                <code>CREATE TABLE</code> + linhas comuns a todo caso). Os dados que variam entre
                casos vão no campo "Dados de Seed" de cada caso de teste, não aqui.
              </p>
              <textarea
                className="input-code input-code--tall"
                aria-label="Schema SQL"
                value={draft.sql_schema}
                onChange={(e) => setDraft({ ...draft, sql_schema: e.target.value })}
                placeholder={'CREATE TABLE students(id INTEGER, name TEXT, grade REAL);'}
                spellCheck={false}
                disabled={saving}
              />
            </section>
          ) : (
            <section className="studio__panel">
              <h2 className="studio__panel-title">Template de Código Inicial</h2>
              <p className="studio__panel-hint">
                O que fica <strong>acima</strong> da linha <code>[[ASCEND::RUNNER]]</code> é o stub
                visível que inicializa o editor do estudante; o que fica <strong>abaixo</strong> é
                o harness oculto de stdin/stdout que o judge concatena à solução na execução. Sem o
                marcador, o snippet inteiro é exibido e a solução roda como enviada. Deixe em
                branco para usar os templates padrão por linguagem.
              </p>
              <textarea
                className="input-code input-code--tall"
                aria-label="Template de Código Inicial"
                value={draft.starter_code}
                onChange={(e) => setDraft({ ...draft, starter_code: e.target.value })}
                placeholder={STARTER_PLACEHOLDER}
                spellCheck={false}
                disabled={saving}
              />
            </section>
          )}
          </>
          )}
        </form>
      </main>
    )
  }

  return (
    <main className="page-shell">
      <section className="hero">
        <p className="eyebrow">Ascend</p>
        <h1>Desafios disponíveis</h1>
        <p className="muted">Escolha um desafio, escreva sua solução e submeta para avaliação.</p>
      </section>

      {isTeacher ? (
        <div className="action-bar">
          <span className="action-bar__label">Área do professor</span>
          <div className="studio__actions">
            <button
              type="button"
              className="btn-secondary"
              onClick={() => setManagingCollections((prev) => !prev)}
            >
              {managingCollections ? 'Fechar coleções' : 'Gerenciar coleções'}
            </button>
            <button
              type="button"
              className="challenge-submit"
              onClick={() => {
                setSaveError(null)
                setCreating(true)
              }}
            >
              Criar desafio
            </button>
          </div>
        </div>
      ) : null}

      {isTeacher && managingCollections ? (
        <div className="collection-manager">
          {collectionActionError ? (
            <p className="status-message status-error">{collectionActionError}</p>
          ) : null}

          <h2 className="collection-manager__title">Grupos</h2>
          {groups.length === 0 ? (
            <p className="studio__panel-hint">Nenhum grupo criado ainda.</p>
          ) : (
            groups.map((cg, index) => (
              <div className="collection-manager__row" key={cg.id}>
                <input
                  value={groupEdits[cg.id] ?? cg.title}
                  onChange={(e) => setGroupEdits((prev) => ({ ...prev, [cg.id]: e.target.value }))}
                  onBlur={() => void handleRenameGroup(cg)}
                  disabled={collectionSaving}
                />
                <div className="list-item__actions">
                  <button
                    type="button"
                    className="list-item__icon-btn"
                    title="Mover para cima"
                    aria-label="Mover grupo para cima"
                    disabled={collectionSaving || index === 0}
                    onClick={() => void handleMoveGroup(index, -1)}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    className="list-item__icon-btn"
                    title="Mover para baixo"
                    aria-label="Mover grupo para baixo"
                    disabled={collectionSaving || index === groups.length - 1}
                    onClick={() => void handleMoveGroup(index, 1)}
                  >
                    ↓
                  </button>
                </div>
              </div>
            ))
          )}
          <div className="collection-manager__create">
            <input
              value={newGroupTitle}
              onChange={(e) => setNewGroupTitle(e.target.value)}
              placeholder="Novo grupo"
              disabled={collectionSaving}
            />
            <button
              type="button"
              className="btn-add"
              onClick={() => void handleCreateGroup()}
              disabled={collectionSaving || newGroupTitle.trim() === ''}
            >
              + Criar grupo
            </button>
          </div>

          <h2 className="collection-manager__title collection-manager__title--spaced">Coleções</h2>
          {collections.length === 0 ? (
            <p className="studio__panel-hint">Nenhuma coleção criada ainda.</p>
          ) : (
            collections.map((cc, index) => (
              <div className="collection-manager__row" key={cc.id}>
                <input
                  value={collectionEdits[cc.id] ?? cc.title}
                  onChange={(e) =>
                    setCollectionEdits((prev) => ({ ...prev, [cc.id]: e.target.value }))
                  }
                  onBlur={() => void handleRenameCollection(cc)}
                  disabled={collectionSaving}
                />
                <select
                  value={cc.group_id ?? ''}
                  onChange={(e) => void handleChangeCollectionGroup(cc, e.target.value)}
                  disabled={collectionSaving}
                  aria-label={`Grupo de ${cc.title}`}
                >
                  <option value="">Sem grupo</option>
                  {groups.map((cg) => (
                    <option key={cg.id} value={cg.id}>
                      {cg.title}
                    </option>
                  ))}
                </select>
                <div className="list-item__actions">
                  <button
                    type="button"
                    className="list-item__icon-btn"
                    title="Mover para cima"
                    aria-label="Mover coleção para cima"
                    disabled={collectionSaving || index === 0}
                    onClick={() => void handleMoveCollection(index, -1)}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    className="list-item__icon-btn"
                    title="Mover para baixo"
                    aria-label="Mover coleção para baixo"
                    disabled={collectionSaving || index === collections.length - 1}
                    onClick={() => void handleMoveCollection(index, 1)}
                  >
                    ↓
                  </button>
                </div>
              </div>
            ))
          )}
          <div className="collection-manager__create">
            <input
              value={newCollectionTitle}
              onChange={(e) => setNewCollectionTitle(e.target.value)}
              placeholder="Nova coleção"
              disabled={collectionSaving}
            />
            <select
              value={newCollectionGroupId}
              onChange={(e) => setNewCollectionGroupId(e.target.value)}
              disabled={collectionSaving}
              aria-label="Grupo da nova coleção"
            >
              <option value="">Sem grupo</option>
              {groups.map((cg) => (
                <option key={cg.id} value={cg.id}>
                  {cg.title}
                </option>
              ))}
            </select>
            <button
              type="button"
              className="btn-add"
              onClick={() => void handleCreateCollection()}
              disabled={collectionSaving || newCollectionTitle.trim() === ''}
            >
              + Criar coleção
            </button>
          </div>
        </div>
      ) : null}

      {loading ? <p className="status-message">Carregando desafios...</p> : null}

      {error ? <p className="status-message status-error">{error}</p> : null}

      {!loading && !error ? (
        challenges.length > 0 ? (
          (() => {
            const entries = groupByGroupAndCollection(challenges)
            const showTopHeaders = entries.length > 1
            return entries.map((entry) => {
              if (entry.kind === 'group') {
                const isGroupCollapsed = collapsedTopGroups.has(entry.key)
                const showNestedHeaders = entry.collections.length > 1
                const totalInGroup = entry.collections.reduce(
                  (sum, bucket) => sum + bucket.challenges.length,
                  0,
                )
                return (
                  <div key={entry.key} className="challenge-group">
                    <button
                      type="button"
                      className="challenge-group-header"
                      aria-expanded={!isGroupCollapsed}
                      onClick={() => toggleTopGroupCollapsed(entry.key)}
                    >
                      <span
                        className={isGroupCollapsed ? 'audit-caret' : 'audit-caret audit-caret--open'}
                        aria-hidden="true"
                      >
                        ▸
                      </span>
                      {entry.label}
                      {isGroupCollapsed ? ` (${totalInGroup})` : null}
                    </button>
                    {/* Always mounted, hidden via CSS — see the same note on
                        .challenge-feed below re: the removeChild crash this
                        avoids. */}
                    <div className="challenge-group-body" hidden={isGroupCollapsed}>
                      {entry.collections.map((bucket) => {
                        const isCollapsed = showNestedHeaders && collapsedGroups.has(bucket.key)
                        return (
                          <div
                            key={bucket.key}
                            className="challenge-collection-group challenge-collection-group--nested"
                          >
                            {showNestedHeaders ? (
                              <button
                                type="button"
                                className="challenge-collection-header challenge-collection-header--nested"
                                aria-expanded={!isCollapsed}
                                onClick={() => toggleGroupCollapsed(bucket.key)}
                              >
                                <span
                                  className={isCollapsed ? 'audit-caret' : 'audit-caret audit-caret--open'}
                                  aria-hidden="true"
                                >
                                  ▸
                                </span>
                                {bucket.label}
                                {isCollapsed ? ` (${bucket.challenges.length})` : null}
                              </button>
                            ) : null}
                            <div className="challenge-feed" hidden={isCollapsed}>
                              {bucket.challenges.map((challenge) => (
                                <ChallengeCard
                                  key={challenge.id}
                                  challenge={challenge}
                                  isTeacher={isTeacher}
                                  onEdit={(c) => void openEditor(c)}
                                />
                              ))}
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )
              }

              const bucket = entry.bucket
              const isCollapsed = showTopHeaders && collapsedGroups.has(bucket.key)
              return (
                <div key={bucket.key} className="challenge-collection-group">
                  {showTopHeaders ? (
                    <button
                      type="button"
                      className="challenge-collection-header"
                      aria-expanded={!isCollapsed}
                      onClick={() => toggleGroupCollapsed(bucket.key)}
                    >
                      <span
                        className={isCollapsed ? 'audit-caret' : 'audit-caret audit-caret--open'}
                        aria-hidden="true"
                      >
                        ▸
                      </span>
                      {bucket.label}
                      {isCollapsed ? ` (${bucket.challenges.length})` : null}
                    </button>
                  ) : null}
                  {/* Always mounted, hidden via CSS rather than conditionally
                      unmounted — React unmounting/remounting this subtree on
                      every collapse toggle is what let a "removeChild: node is
                      not a child of this node" crash surface (a DOM-mutating
                      browser extension, e.g. page translation, can move/replace
                      text nodes in here between renders; if React never has to
                      tear the subtree down, it never fights that mutation). */}
                  <div className="challenge-feed" hidden={isCollapsed}>
                    {bucket.challenges.map((challenge) => (
                      <ChallengeCard
                        key={challenge.id}
                        challenge={challenge}
                        isTeacher={isTeacher}
                        onEdit={(c) => void openEditor(c)}
                      />
                    ))}
                  </div>
                </div>
              )
            })
          })()
        ) : (
          <p className="status-message">Nenhum desafio cadastrado.</p>
        )
      ) : null}
    </main>
  )
}
