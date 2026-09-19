import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { API_BASE_URL, createLiveSession, createLiveSessionRounds, endLiveSessionRound, finishLiveSession, getCurrentLiveSessionRound, getLiveDashboard, getLiveRoundStatus, getLiveSession, joinLiveSession, listLiveSessions, listProblemLists, startLiveSessionRound, type LiveDashboard, type LiveRoundLiveStatus, type LiveSession, type LiveSessionDetail, type LiveSessionRound, type ProblemList } from '../api'
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
  const [mode, setMode] = useState<LiveSession['mode']>('individual')
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
          const firstEligible = ownLists.find((list) => list.live_session_eligible)
          setListID((current) => {
            const currentStillEligible = ownLists.find((list) => list.id === current)?.live_session_eligible
            return currentStillEligible ? current : firstEligible?.id ?? ''
          })
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
    const selected = lists.find((list) => list.id === listID)
    if (!listID || !selected?.live_session_eligible || actionPending) return
    try {
      setActionPending(true)
      const session = await createLiveSession(listID, minimum, mode)
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
      <label>Lista<select value={listID} onChange={(event) => setListID(event.target.value)} required>
        {lists.length === 0 ? <option value="">Nenhuma lista criada ainda</option> : null}
        {lists.map((list) => (
          <option key={list.id} value={list.id} disabled={!list.live_session_eligible}>
            {list.title}{list.live_session_eligible ? '' : ' (sem desafios vinculados — indisponível)'}
          </option>
        ))}
      </select></label>
      <p className="muted">
        Só listas com pelo menos um item vinculado a um desafio executável podem virar uma Live
        Session, porque o acompanhamento ao vivo é feito por submissões ao judge. Vincule um desafio
        a um item na edição da lista para habilitá-la aqui.
      </p>
      <label>Modo<select value={mode} onChange={(event) => setMode(event.target.value as LiveSession['mode'])}>
        <option value="individual">Individual (ritmo livre)</option>
        <option value="timed_challenge">Desafio cronometrado</option>
      </select></label>
      <label>Mínimo de participantes<input type="number" min="1" value={minimum} onChange={(event) => setMinimum(Math.max(1, Number(event.target.value) || 1))} /></label>
      <button className="challenge-submit" disabled={!listID || !lists.find((list) => list.id === listID)?.live_session_eligible || actionPending}>{actionPending ? 'criando...' : 'criar sessão'}</button>
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
    {session.mode === 'timed_challenge' ? (
      isOwner ? <TimedTeacherPanel sessionID={session.id} items={session.items} /> : <TimedStudentPanel sessionID={session.id} items={session.items} />
    ) : isOwner && dashboard ? <Dashboard dashboard={dashboard} /> : null}
  </main>
}

function websocketURL(sessionID: string): string {
  const base = API_BASE_URL ? API_BASE_URL.replace(/^http/, 'ws') : `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}`
  return `${base}/api/v1/live-sessions/${sessionID}/events`
}

function roundTitle(round: LiveSessionRound, items: LiveSession['items']): string {
  return items.find((item) => item.id === round.list_item_id)?.title ?? `Desafio ${round.sequence + 1}`
}

function TimedTeacherPanel({ sessionID, items }: { sessionID: string; items: LiveSession['items'] }) {
  const [rounds, setRounds] = useState<LiveSessionRound[]>([])
  const [liveStatus, setLiveStatus] = useState<LiveRoundLiveStatus | null>(null)
  const [minutes, setMinutes] = useState(10)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const roundsRef = useRef<LiveSessionRound[]>([])

  const loadRounds = useCallback(async () => {
    const configured = await createLiveSessionRounds(sessionID)
    roundsRef.current = configured
    setRounds(configured)
  }, [sessionID])
  const refreshLiveStatus = useCallback(async () => {
    const active = roundsRef.current.find((round) => round.status === 'active')
    if (!active) { setLiveStatus(null); return }
    const next = await getLiveRoundStatus(sessionID, active.id)
    setLiveStatus(next)
    roundsRef.current = roundsRef.current.map((round) => round.id === next.round.id ? next.round : round)
    setRounds(roundsRef.current)
  }, [sessionID])

  useEffect(() => {
    const initial = window.setTimeout(() => void loadRounds().then(refreshLiveStatus).catch((cause: unknown) => setError(cause instanceof Error ? cause.message : 'Falha ao carregar rounds')), 0)
    const timer = window.setInterval(() => void refreshLiveStatus().catch(() => {}), POLL_MS)
    const socket = new WebSocket(websocketURL(sessionID))
    socket.onmessage = (event) => {
      const message = JSON.parse(event.data) as { type?: string }
      if (message.type === 'submission_update') void refreshLiveStatus().catch(() => {})
      else void loadRounds().then(refreshLiveStatus).catch(() => {})
    }
    return () => { window.clearTimeout(initial); window.clearInterval(timer); socket.close() }
  }, [loadRounds, refreshLiveStatus, sessionID])

  async function start(round: LiveSessionRound) {
    try {
      setPending(true); setError(null)
      await startLiveSessionRound(sessionID, round.id, Math.max(1, minutes) * 60)
      await loadRounds(); await refreshLiveStatus()
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Falha ao iniciar round') } finally { setPending(false) }
  }
  async function end(round: LiveSessionRound) {
    try {
      setPending(true); setError(null)
      await endLiveSessionRound(sessionID, round.id)
      await loadRounds(); await refreshLiveStatus()
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Falha ao encerrar round') } finally { setPending(false) }
  }

  const active = rounds.find((round) => round.status === 'active')
  const next = rounds.find((round) => round.status === 'pending')
  return <section className="history-section"><h2 className="section-title">Desafio cronometrado</h2>
    {error ? <p className="status-message status-error">{error}</p> : null}
    {rounds.length === 0 ? <p className="muted">Preparando os desafios da lista...</p> : null}
    <label>Duração da próxima rodada (minutos)<input type="number" min="1" value={minutes} onChange={(event) => setMinutes(Math.max(1, Number(event.target.value) || 1))} /></label>
    <div className="list-items">{rounds.map((round) => <article className="list-item" key={round.id}><div className="list-item__head"><div><strong>{round.sequence + 1}. {roundTitle(round, items)}</strong><p className="muted">{round.status === 'pending' ? 'aguardando' : round.status === 'active' ? 'em andamento' : 'encerrada'}</p></div>
      {round.status === 'pending' && !active ? <button className="challenge-submit" disabled={pending || round.id !== next?.id} onClick={() => void start(round)}>iniciar</button> : null}
      {round.status === 'active' ? <button className="btn-secondary" disabled={pending} onClick={() => void end(round)}>encerrar agora</button> : null}
    </div></article>)}</div>
    {liveStatus ? <RoundStatusDashboard liveStatus={liveStatus} /> : null}
  </section>
}

function RoundStatusDashboard({ liveStatus }: { liveStatus: LiveRoundLiveStatus }) {
  const label: Record<LiveRoundLiveStatus['students'][number]['status'], string> = { not_started: 'não iniciou', submitted_pending: 'em correção', passed: 'passou', failed: 'não passou' }
  const color: Record<LiveRoundLiveStatus['students'][number]['status'], string> = { not_started: '', submitted_pending: 'verdict--pending', passed: 'verdict--accepted', failed: 'verdict--wrong_answer' }
  return <div className="table-wrap"><table><thead><tr><th>Aluno</th><th>Status</th></tr></thead><tbody>{liveStatus.students.map((student) => <tr key={student.user_id}><td>{student.email}</td><td><span className={`verdict ${color[student.status]}`}>{label[student.status]}</span></td></tr>)}</tbody></table></div>
}

function TimedStudentPanel({ sessionID, items }: { sessionID: string; items: LiveSession['items'] }) {
  const [round, setRound] = useState<LiveSessionRound | null>(null)
  const [now, setNow] = useState(0)
  useEffect(() => {
    const refresh = () => void getCurrentLiveSessionRound(sessionID).then(setRound).catch(() => setRound(null))
    refresh()
    const poll = window.setInterval(refresh, POLL_MS)
    const tick = () => setNow(Date.now())
    tick()
    const ticker = window.setInterval(tick, 1000)
    const socket = new WebSocket(websocketURL(sessionID))
    socket.onmessage = refresh
    return () => { window.clearInterval(poll); window.clearInterval(ticker); socket.close() }
  }, [sessionID])
  if (!round) return <section className="panel submission-panel"><div className="submission-panel__body"><h2>Aguardando o professor</h2><p className="muted">O próximo desafio aparecerá aqui quando a rodada começar.</p></div></section>
  const item = items.find((candidate) => candidate.id === round.list_item_id)
  const endsAt = round.started_at ? new Date(round.started_at).getTime() + round.duration_seconds * 1000 : 0
  const secondsLeft = Math.max(0, Math.ceil((endsAt - now) / 1000))
  const isActive = round.status === 'active' && secondsLeft > 0
  return <section className="panel submission-panel"><div className="submission-panel__body"><h2>{roundTitle(round, items)}</h2><p className="muted">{isActive ? `Tempo restante: ${Math.floor(secondsLeft / 60)}:${String(secondsLeft % 60).padStart(2, '0')}` : 'Esta rodada foi encerrada.'}</p>
    {item?.linked_challenge_id ? <Link className="challenge-submit" to={`/challenges/${item.linked_challenge_id}?liveSessionId=${sessionID}`}>{isActive ? 'resolver desafio' : 'ver desafio'}</Link> : <p className="status-message status-error">Este item não possui desafio vinculado.</p>}
  </div></section>
}

function Dashboard({ dashboard }: { dashboard: LiveDashboard }) {
  const items = dashboard.session.items
  return <section className="history-section"><h2 className="section-title">Acompanhamento ao vivo</h2><div className="table-wrap"><table><thead><tr><th>Aluno</th><th>Total</th><th>Aceitos</th>{items.map((item, index) => <th key={item.id}>C{index + 1}</th>)}</tr></thead><tbody>{dashboard.students.map((student) => <tr key={student.user_id}><td>{student.email}</td><td>{student.attempts}</td><td>{student.accepted}</td>{items.map((item) => { const challenge = student.challenges[item.linked_challenge_id ?? '']; return <td key={item.id}>{!challenge ? '—' : challenge.accepted ? `✓ (${challenge.attempts})` : `✗ (${challenge.attempts})`}</td> })}</tr>)}</tbody></table></div><p className="muted">Por desafio: {items.map((item, index) => `C${index + 1}: ${dashboard.challenge_accepted[item.linked_challenge_id ?? ''] ?? 0} aceitos / ${dashboard.challenge_attempts[item.linked_challenge_id ?? ''] ?? 0} tentativas`).join(' · ')}</p></section>
}
