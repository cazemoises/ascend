import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { createLiveSession, finishLiveSession, getLiveDashboard, getLiveSession, joinLiveSession, listLiveSessions, listProblemLists, type LiveDashboard, type LiveSession, type LiveSessionDetail, type ProblemList } from '../api'
import { useAuth } from '../auth/useAuth'

const POLL_MS = 2000

function statusText(status: LiveSession['status']): string {
  return status === 'waiting' ? 'aguardando participantes' : status === 'active' ? 'em andamento' : 'encerrada'
}

export function LiveSessionsPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { isTeacher, user } = useAuth()
  const [sessions, setSessions] = useState<LiveSession[]>([])
  const [lists, setLists] = useState<ProblemList[]>([])
  const [listID, setListID] = useState('')
  const [minimum, setMinimum] = useState(3)
  const [detail, setDetail] = useState<LiveSessionDetail | null>(null)
  const [dashboard, setDashboard] = useState<LiveDashboard | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionPending, setActionPending] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
	  setError(null)
      if (id) {
        const nextDetail = await getLiveSession(id)
        setDetail(nextDetail)
        const isOwner = isTeacher && nextDetail.session.created_by === user?.id
        if (isOwner && nextDetail.session.status !== 'finished') setDashboard(await getLiveDashboard(id))
        else setDashboard(null)
		return nextDetail.session.status !== 'finished'
      } else {
        setSessions(await listLiveSessions())
        if (isTeacher) {
          const ownLists = (await listProblemLists()).filter((item) => item.teacher_id === user?.id)
          setLists(ownLists)
          setListID((current) => current || ownLists[0]?.id || '')
        }
		return true
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Falha ao carregar sessões')
		return true
    } finally {
      setLoading(false)
    }
  }, [id, isTeacher, user])

  useEffect(() => {
    let cancelled = false
    let timer: number | undefined
    const poll = async () => {
      const shouldContinue = await load()
      if (!cancelled && shouldContinue) timer = window.setTimeout(() => void poll(), POLL_MS)
    }
    void poll()
    return () => { cancelled = true; if (timer !== undefined) window.clearTimeout(timer) }
  }, [load])

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!listID || actionPending) return
    try {
      setActionPending(true)
      const session = await createLiveSession(listID, minimum)
      navigate(`/sessoes/${session.id}`)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Falha ao criar sessão')
    } finally { setActionPending(false) }
  }

  async function join() {
    if (!id || actionPending) return
    try { setActionPending(true); await joinLiveSession(id); await load() }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Falha ao entrar') }
    finally { setActionPending(false) }
  }

  async function finish() {
    if (!id || actionPending || !window.confirm('Encerrar esta sessão?')) return
    try { setActionPending(true); await finishLiveSession(id); await load() }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Falha ao encerrar') }
    finally { setActionPending(false) }
  }

  if (!id) return <main className="page-shell">
    <section className="hero"><p className="eyebrow">Atividade em grupo</p><h1>sessões ao vivo</h1><p className="muted">Entre em uma atividade ou conduza sua turma.</p></section>
    {error ? <p className="status-message status-error">{error}</p> : null}
    {isTeacher ? <section className="panel submission-panel"><div className="submission-panel__body"><h2>Nova sessão</h2><form onSubmit={create}>
      <label>Lista<select value={listID} onChange={(event) => setListID(event.target.value)} required>{lists.map((list) => <option key={list.id} value={list.id}>{list.title}</option>)}</select></label>
      <label>Mínimo de participantes<input type="number" min="1" value={minimum} onChange={(event) => setMinimum(Math.max(1, Number(event.target.value) || 1))} /></label>
      <button className="challenge-submit" disabled={!listID || actionPending}>{actionPending ? 'criando...' : 'criar sessão'}</button>
    </form></div></section> : null}
    <section className="history-section"><h2 className="section-title">{isTeacher ? 'Suas sessões abertas' : 'Disponíveis'}</h2>
      {loading ? <p className="muted">Carregando sessões...</p> : null}
      {sessions.map((session) => <article className="list-item" key={session.id}><div className="list-item__head"><div><strong>{session.title}</strong><p className="muted">Professor: {session.teacher_email} · {statusText(session.status)} · {session.participant_count}/{session.min_participants}</p></div><Link className="challenge-submit" to={`/sessoes/${session.id}`}>abrir</Link></div></article>)}
      {!loading && sessions.length === 0 ? <p className="muted">Nenhuma sessão aberta.</p> : null}
    </section>
  </main>

  if (loading && !detail) return <main className="page-shell"><p>Carregando sessão...</p></main>
  if (!detail) return <main className="page-shell"><p className="status-message status-error">{error ?? 'Sessão não encontrada.'}</p></main>

  const session = detail.session
  const isOwner = isTeacher && session.created_by === user?.id
  const canJoin = !isTeacher && !session.joined && session.status !== 'finished'
  const canSeeExercises = session.joined || isOwner
  return <main className="page-shell">
    <Link className="back-link" to="/sessoes">← sessões ao vivo</Link>
    <section className="hero"><p className="eyebrow">Atividade em grupo</p><h1>{session.title}</h1><p className="muted">{statusText(session.status)} · {session.participant_count}/{session.min_participants} participantes</p>
      {canJoin ? <button className="challenge-submit" onClick={() => void join()} disabled={actionPending}>{actionPending ? 'entrando...' : 'entrar na sessão'}</button> : null}
      {isOwner && session.status !== 'finished' ? <button className="btn-secondary" onClick={() => void finish()} disabled={actionPending}>{actionPending ? 'encerrando...' : 'encerrar sessão'}</button> : null}
    </section>
    {error ? <p className="status-message status-error">{error}</p> : null}
    <section className="panel submission-panel"><div className="submission-panel__body"><h2>{session.status === 'waiting' ? 'Lobby' : 'Desafios'}</h2>
      {!canSeeExercises ? <p className="muted">Entre na sessão para acessar os desafios e acompanhar a turma.</p> : null}
      {canSeeExercises && session.status === 'waiting' ? <><p className="muted">A atividade começará automaticamente quando atingir o mínimo.</p><ul>{detail.participants.map((participant) => <li key={participant.user_id}>{participant.email}</li>)}</ul></> : null}
      {canSeeExercises && session.status !== 'waiting' ? <div className="list-items">{session.items.map((item, index) => <article className="list-item" key={item.id}><div className="list-item__head"><span>{index + 1}. {item.title}</span><Link className="challenge-submit" to={`/challenges/${item.linked_challenge_id}?liveSessionId=${session.id}`}>{session.status === 'finished' ? 'ver desafio' : 'resolver'}</Link></div></article>)}</div> : null}
    </div></section>
    {isOwner && dashboard ? <Dashboard dashboard={dashboard} /> : null}
  </main>
}

function Dashboard({ dashboard }: { dashboard: LiveDashboard }) {
  const items = dashboard.session.items
  return <section className="history-section"><h2 className="section-title">Acompanhamento ao vivo</h2><div className="table-wrap"><table><thead><tr><th>Aluno</th><th>Total</th><th>Aceitos</th>{items.map((item, index) => <th key={item.id}>C{index + 1}</th>)}</tr></thead><tbody>{dashboard.students.map((student) => <tr key={student.user_id}><td>{student.email}</td><td>{student.attempts}</td><td>{student.accepted}</td>{items.map((item) => { const challenge = student.challenges[item.linked_challenge_id ?? '']; return <td key={item.id}>{!challenge ? '—' : challenge.accepted ? `✓ (${challenge.attempts})` : `✗ (${challenge.attempts})`}</td> })}</tr>)}</tbody></table></div><p className="muted">Por desafio: {items.map((item, index) => `C${index + 1}: ${dashboard.challenge_accepted[item.linked_challenge_id ?? ''] ?? 0} aceitos / ${dashboard.challenge_attempts[item.linked_challenge_id ?? ''] ?? 0} tentativas`).join(' · ')}</p></section>
}
