import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'

import {
  createChallenge,
  listChallenges,
  listTestCases,
  replaceTestCases,
  updateChallenge,
  type Challenge,
  type ChallengeDifficulty,
} from '../api'
import { useAuth } from '../auth/useAuth'
import { TelemetryChip } from '../components/TelemetryChip'

type TestCaseDraft = {
  input: string
  expected_output: string
  is_sample: boolean
}

const EMPTY_DRAFT = {
  slug: '',
  title: '',
  description: '',
  difficulty: 'easy' as ChallengeDifficulty,
  time_limit_ms: '2000',
  memory_limit_mb: '256',
  starter_code: '',
}

const EMPTY_TEST_CASE: TestCaseDraft = {
  input: '',
  expected_output: '',
  is_sample: false,
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

  const [creating, setCreating] = useState(false)
  const [editingChallengeId, setEditingChallengeId] = useState<string | null>(null)
  const [draft, setDraft] = useState(EMPTY_DRAFT)
  const [testCases, setTestCases] = useState<TestCaseDraft[]>([{ ...EMPTY_TEST_CASE }])
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

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
      })
      setTestCases(
        cases.length > 0
          ? cases.map((tc) => ({
              input: tc.input,
              expected_output: tc.expected_output,
              is_sample: tc.is_sample,
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
      }
      const suiteInput = suite.map((tc) => ({
        input: tc.input,
        expected_output: tc.expected_output,
        is_sample: tc.is_sample,
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
                      Entrada Padrão (stdin)
                      <textarea
                        className="input-code"
                        value={tc.input}
                        onChange={(e) => updateTestCase(index, { input: e.target.value })}
                        placeholder={'2 3'}
                        disabled={saving}
                      />
                    </label>
                    <label>
                      Saída Esperada (stdout)
                      <textarea
                        className="input-code"
                        value={tc.expected_output}
                        onChange={(e) =>
                          updateTestCase(index, { expected_output: e.target.value })
                        }
                        placeholder={'5'}
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

          <section className="studio__panel">
            <h2 className="studio__panel-title">Template de Código Inicial</h2>
            <p className="studio__panel-hint">
              O que fica <strong>acima</strong> da linha <code>[[ASCEND::RUNNER]]</code> é o stub
              visível que inicializa o editor do estudante; o que fica <strong>abaixo</strong> é o
              harness oculto de stdin/stdout que o judge concatena à solução na execução. Sem o
              marcador, o snippet inteiro é exibido e a solução roda como enviada. Deixe em branco
              para usar os templates padrão por linguagem.
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
          <div className="challenge-feed">
            {challenges.map((challenge) => (
              <article key={challenge.id} className="challenge-row">
                <span className={`difficulty difficulty--${challenge.difficulty}`}>
                  {challenge.difficulty}
                </span>
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
        ) : (
          <p className="status-message">Nenhum desafio cadastrado.</p>
        )
      ) : null}
    </main>
  )
}
