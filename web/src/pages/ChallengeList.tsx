import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'

import {
  createChallenge,
  importChallenges,
  listChallengeCollections,
  listChallenges,
  listTestCases,
  replaceTestCases,
  updateChallenge,
  type Challenge,
  type ChallengeCollection,
  type ChallengeDifficulty,
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

type CollectionGroup = {
  key: string
  label: string
  challenges: Challenge[]
}

// Groups consecutive-by-key challenges into collection buckets, in the
// order they arrive — the backend already sorts by collection.ordinal then
// created_at, so this only detects group boundaries, it doesn't re-sort.
// Uncategorized challenges land in one trailing "SEM COLEÇÃO" bucket.
function groupByCollection(challenges: Challenge[]): CollectionGroup[] {
  const groups: CollectionGroup[] = []
  const byKey = new Map<string, CollectionGroup>()

  for (const challenge of challenges) {
    const key = challenge.collection_id ?? 'uncategorized'
    const label = challenge.collection_id ? (challenge.collection_title ?? '') : 'SEM COLEÇÃO'

    let group = byKey.get(key)
    if (!group) {
      group = { key, label, challenges: [] }
      byKey.set(key, group)
      groups.push(group)
    }
    group.challenges.push(challenge)
  }

  return groups
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
      const created = await importChallenges(parsed as ImportChallengesInput)
      setChallenges((prev) => [
        ...created.map((c) => ({
          ...c,
          sample_test_cases: c.test_cases
            .filter((tc) => tc.is_sample)
            .map((tc) => ({
              input: tc.input,
              expected_output: tc.expected_output,
              ordinal: tc.ordinal,
            })),
        })),
        ...prev,
      ])
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
        const updated = await updateChallenge(editingChallengeId, payload)
        const savedCases = await replaceTestCases(editingChallengeId, suiteInput)
        const samples = savedCases
          .filter((tc) => tc.is_sample)
          .map((tc) => ({
            input: tc.input,
            expected_output: tc.expected_output,
            ordinal: tc.ordinal,
          }))
        setChallenges((prev) =>
          prev.map((c) =>
            c.id === editingChallengeId ? { ...updated, sample_test_cases: samples } : c,
          ),
        )
        closeStudio()
        return
      }

      const created = await createChallenge(payload)
      try {
        const savedCases = await replaceTestCases(created.id, suiteInput)
        const samples = savedCases
          .filter((tc) => tc.is_sample)
          .map((tc) => ({
            input: tc.input,
            expected_output: tc.expected_output,
            ordinal: tc.ordinal,
          }))
        setChallenges((prev) => [{ ...created, sample_test_cases: samples }, ...prev])
      } catch (err) {
        setChallenges((prev) => [{ ...created, sample_test_cases: [] }, ...prev])
        setSaveError(
          'O desafio foi publicado, mas a suíte de testes falhou ao salvar' +
            (err instanceof Error ? ` (${err.message})` : '') +
            '. Use o botão Editar do desafio para reenviá-la.',
        )
        return
      }

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
      ) : null}

      {loading ? <p className="status-message">Carregando desafios...</p> : null}

      {error ? <p className="status-message status-error">{error}</p> : null}

      {!loading && !error ? (
        challenges.length > 0 ? (
          (() => {
            const groups = groupByCollection(challenges)
            const showGroupHeaders = groups.length > 1
            return groups.map((group) => (
              <div key={group.key} className="challenge-collection-group">
                {showGroupHeaders ? (
                  <h2 className="challenge-collection-header">{group.label}</h2>
                ) : null}
                <div className="challenge-feed">
                  {group.challenges.map((challenge) => (
                    <article key={challenge.id} className="challenge-row">
                      <div className="challenge-row__badges">
                        <span className={`difficulty difficulty--${challenge.difficulty}`}>
                          {challenge.difficulty}
                        </span>
                        {challenge.solved ? (
                          <span className="verdict verdict--accepted">CONCLUÍDO</span>
                        ) : null}
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
                            onClick={() => void openEditor(challenge)}
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
                  ))}
                </div>
              </div>
            ))
          })()
        ) : (
          <p className="status-message">Nenhum desafio cadastrado.</p>
        )
      ) : null}
    </main>
  )
}
