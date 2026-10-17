import { useEffect, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, getAssessmentAuthoring, getProblemList, saveAssessment, type AssessmentAuthoring, type AssessmentKind, type ListItem, type ProblemListDetail } from '../api'
import { useAuth } from '../auth/useAuth'

function ExerciseEditor({ item, initial }: { item: ListItem; initial: AssessmentAuthoring | null }) {
  const [kind, setKind] = useState<AssessmentKind>(initial?.kind ?? 'multiple_choice')
  const [prompt, setPrompt] = useState(initial?.prompt ?? item.body)
  const [optionsText, setOptionsText] = useState(initial?.options.join('\n') ?? '')
  const [choice, setChoice] = useState(initial?.kind === 'multiple_choice' ? initial.accepted_answers[0] : '')
  const [answersText, setAnswersText] = useState(initial?.kind === 'short_answer' ? initial.accepted_answers.join('\n') : '')
  const [caseSensitive, setCaseSensitive] = useState(initial?.case_sensitive ?? false)
  const [revision, setRevision] = useState(initial?.revision ?? 0)
  const [pending, setPending] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const options = optionsText.split('\n').map((option) => option.trim()).filter(Boolean)
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      setPending(true); setError(null); setMessage(null)
      const saved = await saveAssessment(item.id, { kind, prompt, options: kind === 'multiple_choice' ? options : [], accepted_answers: kind === 'multiple_choice' ? [choice] : answersText.split('\n').map((answer) => answer.trim()).filter(Boolean), case_sensitive: kind === 'short_answer' && caseSensitive })
      setRevision(saved.revision)
      setMessage(`Versão ${saved.revision} salva. As tentativas anteriores foram preservadas.`)
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Falha ao salvar exercício') }
    finally { setPending(false) }
  }
  return <article className="panel student-card"><h2>{item.title}</h2>
    {item.linked_challenge_id ? <p className="muted">Este item já é corrigido pelo judge de código.</p> : <form onSubmit={save} className="student-card__section">
      <p className="muted">{revision ? `Versão atual: ${revision}` : 'Criar exercício com correção automática'}. Alterar a questão ou o gabarito cria uma nova versão; a conclusão passa a considerar essa versão. Salvar sem alterações mantém a versão.</p>
      <label>Tipo<select aria-label="Tipo" value={kind} onChange={(event) => setKind(event.target.value as AssessmentKind)}><option value="multiple_choice">Múltipla escolha</option><option value="short_answer">Resposta curta</option></select></label>
      <label>Enunciado do exercício<textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} maxLength={20000} required rows={5} /></label>
      {kind === 'multiple_choice' ? <>
        <label>Alternativas (uma por linha, de 2 a 10)<textarea value={optionsText} onChange={(event) => setOptionsText(event.target.value)} rows={5} required /></label>
        <label>Alternativa correta<select aria-label="Alternativa correta" value={choice} onChange={(event) => setChoice(event.target.value)} required><option value="">Selecione</option>{options.map((option, index) => <option key={index} value={option}>{option}</option>)}</select></label>
      </> : <>
        <label>Respostas aceitas (uma por linha, até 10)<textarea value={answersText} onChange={(event) => setAnswersText(event.target.value)} rows={4} required /></label>
        <label><input type="checkbox" checked={caseSensitive} onChange={(event) => setCaseSensitive(event.target.checked)} />Diferenciar maiúsculas e minúsculas</label>
        <p className="muted">Espaços adicionais são desconsiderados; acentos são preservados. A comparação é textual.</p>
      </>}
      <button className="challenge-submit" disabled={pending || (kind === 'multiple_choice' && !options.includes(choice))}>{pending ? 'salvando...' : 'salvar exercício'}</button>
      {message ? <p role="status">{message}</p> : null}
      {error ? <p className="status-message status-error">{error}</p> : null}
    </form>}
  </article>
}

export function AssessmentAuthoringPage() {
  const { id } = useParams<{ id: string }>()
  const { user } = useAuth()
  const [detail, setDetail] = useState<ProblemListDetail | null>(null)
  const [configs, setConfigs] = useState<Record<string, AssessmentAuthoring | null>>({})
  const [error, setError] = useState<string | null>(null)
  useEffect(() => {
    let active = true
    async function load() {
      if (!id || !user) return
      try {
        const list = await getProblemList(id)
        if (list.teacher_id !== user.id) throw new Error('Somente o mentor proprietário pode configurar estes exercícios.')
        const entries = await Promise.all(list.items.map(async (item): Promise<[string, AssessmentAuthoring | null]> => {
          if (!item.assessment_revision_id) return [item.id, null]
          try { return [item.id, await getAssessmentAuthoring(item.id)] }
          catch (cause) { if (cause instanceof ApiError && cause.status === 404) return [item.id, null]; throw cause }
        }))
        if (active) { setDetail(list); setConfigs(Object.fromEntries(entries)) }
      } catch (cause) { if (active) setError(cause instanceof Error ? cause.message : 'Falha ao carregar lista') }
    }
    void load()
    return () => { active = false }
  }, [id, user])
  return <main className="page-shell"><Link className="back-link" to={`/listas/${id}`}>← lista</Link>
    <section className="hero"><p className="eyebrow">Área do mentor</p><h1>correção automática</h1><p className="muted">{detail?.title}. Gabaritos visíveis somente para o proprietário. O enunciado corrigido é editado aqui; a edição comum da lista mantém esta configuração.</p></section>
    {error ? <p className="status-message status-error">{error}</p> : null}
    {!detail && !error ? <p>Carregando exercícios...</p> : null}
    {detail?.items.length === 0 ? <p>Adicione itens na edição da lista para configurar exercícios.</p> : null}
    {detail?.items.map((item) => <ExerciseEditor key={item.id} item={item} initial={configs[item.id] ?? null} />)}
  </main>
}
