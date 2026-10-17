import { useEffect, useState, type FormEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import { ApiError, answerAssessment, getAssessment, getAssessmentAttempts, type Assessment, type AssessmentAttempt } from '../api'
import { useAuth } from '../auth/useAuth'

export function AssessmentCard({ itemID, onAccepted }: { itemID: string; onAccepted: () => void }) {
  const { user } = useAuth()
  const canAnswer = user?.real_role === 'student' && user.effective_role === 'student'
  const [exercise, setExercise] = useState<Assessment | null>(null)
  const [attempts, setAttempts] = useState<AssessmentAttempt[]>([])
  const [answer, setAnswer] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [stale, setStale] = useState(false)
  const [feedback, setFeedback] = useState<boolean | null>(null)
  useEffect(() => {
    let active = true
    void Promise.all([getAssessment(itemID), canAnswer ? getAssessmentAttempts(itemID) : Promise.resolve([])])
      .then(([next, history]) => { if (active) { setExercise(next); setAttempts(history) } })
      .catch((cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : 'Falha ao carregar exercício') })
    return () => { active = false }
  }, [itemID, canAnswer])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!exercise || pending) return
    try {
      setPending(true); setError(null)
      const attempt = await answerAssessment(itemID, exercise.id, answer)
      setAttempts((current) => [attempt, ...current].slice(0, 50))
      setFeedback(attempt.accepted)
      if (attempt.accepted) onAccepted()
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 409) {
        setStale(true); setError('O exercício foi atualizado pelo mentor. Carregue a versão atual para responder.')
      } else setError(cause instanceof Error ? cause.message : 'Falha ao corrigir resposta')
    }
    finally { setPending(false) }
  }
  return <section className="student-card__section" aria-label="Exercício com correção automática">
    {error ? <p className="status-message status-error">{error}</p> : null}
    {stale ? <button className="btn-secondary" onClick={() => window.location.reload()}>carregar versão atual</button> : null}
    {!exercise && !error ? <p>Carregando exercício...</p> : null}
    {exercise ? <>
      <p className="muted">{exercise.kind === 'multiple_choice' ? 'Múltipla escolha' : 'Resposta curta'} · versão {exercise.revision}</p>
      <div className="list-item__body markdown-body"><ReactMarkdown>{exercise.prompt}</ReactMarkdown></div>
      {exercise.kind === 'short_answer' ? <p className="muted">Espaços adicionais são desconsiderados. {exercise.case_sensitive ? 'Maiúsculas e minúsculas são diferenciadas.' : 'Maiúsculas e minúsculas são equivalentes.'} A resposta deve corresponder a uma das alternativas de texto previstas pelo mentor.</p> : null}
      {canAnswer ? <form onSubmit={submit}>
        {exercise.kind === 'multiple_choice' ? <fieldset><legend>Escolha uma resposta</legend>{exercise.options.map((option, index) => <label key={index} className="student-card__row"><input type="radio" name={`assessment-${itemID}`} value={option} checked={answer === option} onChange={() => setAnswer(option)} required />{option}</label>)}</fieldset> : <label>Sua resposta<input value={answer} onChange={(event) => setAnswer(event.target.value)} maxLength={2000} required /></label>}
        <button className="challenge-submit" disabled={pending || stale || !answer.trim()}>{pending ? 'corrigindo...' : 'enviar resposta'}</button>
      </form> : <p className="muted">Entre com uma conta de aluno para responder.</p>}
      {feedback !== null ? <p role="status">{feedback ? 'Resposta aceita. Exercício concluído.' : 'Resposta ainda não aceita. Revise o enunciado e tente novamente.'}</p> : null}
      {canAnswer ? <details><summary>Suas últimas 50 tentativas ({attempts.length})</summary>
        {attempts.length === 0 ? <p className="muted">Nenhuma resposta enviada.</p> : <ul>{attempts.map((attempt) => <li key={attempt.id}>{attempt.accepted ? 'Aceita' : 'Não aceita'} · versão {attempt.exercise.revision} · {new Date(attempt.created_at).toLocaleString('pt-BR')}<p>{attempt.exercise.prompt}</p><p>Sua resposta: {attempt.answer}</p></li>)}</ul>}
      </details> : null}
    </> : null}
  </section>
}
