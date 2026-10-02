import Editor, { type OnMount } from '@monaco-editor/react'
import { useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { MonacoBinding } from 'y-monaco'
import { Awareness } from 'y-protocols/awareness'
import * as Y from 'yjs'

import {
  forceEndLiveRoom,
  getLiveRoomResult,
  getLiveSession,
  liveRoomWebsocketURL,
  listLiveRooms,
  markLiveRoomDone,
  type LiveRoomResult,
  type LiveRoomSummary,
  type LiveSessionDetail,
} from '../api'
import { useAuth } from '../auth/useAuth'
import { VerdictBadge } from '../components/VerdictBadge'
import { registerAscendSnippets } from '../lib/monacoSnippets'
import { defineAscendMonacoTheme } from '../lib/monacoTheme'
import { connectLiveRoom } from '../lib/liveRoomConnection'

export function LiveRoomPage() {
  const { id: sessionID, itemId: listItemID } = useParams<{ id: string; itemId: string }>()
  const { user, isRealTeacher } = useAuth()
  const [session, setSession] = useState<LiveSessionDetail | null>(null)
  const [rooms, setRooms] = useState<LiveRoomSummary[]>([])
  const [connected, setConnected] = useState(false)
  const [frozen, setFrozen] = useState(false)
  const [result, setResult] = useState<LiveRoomResult | null>(null)
  const [markedDone, setMarkedDone] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const ydocRef = useRef<Y.Doc | null>(null)
  const bindingRef = useRef<MonacoBinding | null>(null)

  useEffect(() => {
    if (!sessionID || !listItemID) return
    let cancelled = false
    void getLiveSession(sessionID).then((detail) => { if (!cancelled) setSession(detail) }).catch(() => {})
    void listLiveRooms(sessionID).then((list) => {
      if (cancelled) return
      setRooms(list)
      const summary = list.find((r) => r.list_item_id === listItemID)
      if (summary?.status === 'frozen') {
        setFrozen(true)
        void getLiveRoomResult(sessionID, listItemID).then(setResult).catch(() => {})
      }
    }).catch(() => {})
    return () => { cancelled = true }
  }, [sessionID, listItemID])

  useEffect(() => {
    if (!sessionID || !listItemID) return
    let cancelled = false
    const ydoc = new Y.Doc()
    ydocRef.current = ydoc
    const connection = connectLiveRoom(ydoc, liveRoomWebsocketURL(sessionID, listItemID),
      (value) => { if (!cancelled) setConnected(value) },
      () => {
        if (cancelled) return
        setFrozen(true)
        void getLiveRoomResult(sessionID, listItemID).then(setResult).catch(() => {})
      })

    return () => {
      cancelled = true
      connection.close()
      bindingRef.current?.destroy()
      bindingRef.current = null
      ydoc.destroy()
      ydocRef.current = null
    }
  }, [sessionID, listItemID])

  const handleMount: OnMount = (editorInstance) => {
    const ydoc = ydocRef.current
    const model = editorInstance.getModel()
    if (!ydoc || !model) return
    const ytext = ydoc.getText('code')
    bindingRef.current = new MonacoBinding(ytext, model, new Set([editorInstance]), new Awareness(ydoc))
  }

  async function markDone() {
    if (!sessionID || !listItemID || pending) return
    try {
      setPending(true); setError(null)
      await markLiveRoomDone(sessionID, listItemID)
      setMarkedDone(true)
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Falha ao marcar como concluído') }
    finally { setPending(false) }
  }

  async function forceEnd() {
    if (!sessionID || !listItemID || pending || !window.confirm('Encerrar esta sala para todos?')) return
    try {
      setPending(true); setError(null)
      await forceEndLiveRoom(sessionID, listItemID)
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Falha ao encerrar a sala') }
    finally { setPending(false) }
  }

  if (!sessionID || !listItemID) return null

  const item = session?.session.items.find((candidate) => candidate.id === listItemID)
  const isOwner = isRealTeacher && session?.session.created_by === user?.id
  const summary = rooms.find((r) => r.list_item_id === listItemID)
  const readOnly = frozen

  return <main className="page-shell">
    <Link className="back-link" to={`/sessoes/${sessionID}`}>← {session?.session.title ?? 'sessão'}</Link>
    <section className="hero">
      <p className="eyebrow">Sala colaborativa</p>
      <h1>{item?.title ?? 'Desafio'}</h1>
      <p className="muted">
        {frozen ? 'Sala encerrada' : connected ? 'conectado' : 'conectando...'}
        {summary ? ` · ${summary.present_count} presente(s)` : ''}
      </p>
      {!isRealTeacher && !frozen ? (
        <button className="challenge-submit" disabled={pending || markedDone} onClick={() => void markDone()}>
          {markedDone ? 'aguardando o restante da turma...' : pending ? 'enviando...' : 'terminei'}
        </button>
      ) : null}
      {isOwner && !frozen ? (
        <button className="btn-secondary" disabled={pending} onClick={() => void forceEnd()}>
          {pending ? 'encerrando...' : 'encerrar sala agora'}
        </button>
      ) : null}
    </section>
    {error ? <p className="status-message status-error">{error}</p> : null}
    <div className="workspace__editor" style={{ height: '70vh' }}>
      <div className="editor-toolbar"><span className="editor-toolbar__title">Editor compartilhado</span></div>
      <div className="editor-host">
        <Editor
          key={`${sessionID}:${listItemID}`}
          height="100%"
          theme="ascend"
          beforeMount={(monaco) => { defineAscendMonacoTheme(monaco); registerAscendSnippets(monaco) }}
          onMount={handleMount}
          defaultLanguage="python"
          defaultValue=""
          options={{
            fontFamily: 'JetBrains Mono, Consolas, monospace',
            fontSize: 13,
            minimap: { enabled: false },
            automaticLayout: true,
            padding: { top: 12 },
            readOnly,
            quickSuggestions: { other: true, comments: false, strings: false },
            wordBasedSuggestions: 'matchingDocuments',
            snippetSuggestions: 'inline',
          }}
        />
      </div>
    </div>
    {frozen && result ? (
      <section className="panel submission-panel"><div className="submission-panel__body">
        <h2>Resultado</h2>
        {result.submission ? <>
          <p><VerdictBadge status={result.submission.status} /></p>
          {result.submission.stderr ? <pre className="result-panel__body">{result.submission.stderr}</pre> : null}
        </> : <p className="muted">Este item não possui gabarito — sem correção automática, revise o código final acima.</p>}
      </div></section>
    ) : null}
  </main>
}
