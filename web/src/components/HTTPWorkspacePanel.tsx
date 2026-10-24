import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { Challenge, HTTPRequest } from '../api'
import { initialRequests, serializeRequests, type RequestDraft } from '../lib/httpClient'
import type { useHTTPExecution } from '../lib/useHTTPExecution'
import { SubmissionResult } from './SubmissionResult'

export function HTTPWorkspacePanel({ challenge, execution, onExplore, locked }: {
  challenge: Challenge; execution: ReturnType<typeof useHTTPExecution>;
  onExplore: (requests: HTTPRequest[]) => void; locked: boolean
}) {
  const { tab, setTab } = execution
  const [requests, setRequests] = useState(() => initialRequests(challenge.sample_test_cases))
  const [selected, setSelected] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const draft = requests[selected]
  const disabled = execution.pending || locked
  function update(patch: Partial<RequestDraft>) {
    setRequests(previous => previous.map((request, index) => index === selected ? { ...request, ...patch } : request))
    setError(null)
  }
  function explore() {
    try { const batch = serializeRequests(requests); setError(null); setTab('results'); onExplore(batch) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Requisição inválida') }
  }
  const showResults = tab === 'results'
  return <section className="workspace__insights http-workbench" aria-label="Cliente HTTP e resultados">
    <div className="http-workbench__toolbar"><div className="studio__mode-tabs" role="tablist" aria-label="Painel de execução">
      {(['client', 'results'] as const).map(value => <button type="button" role="tab" id={`http-tab-${value}`}
        aria-controls={`http-panel-${value}`} aria-selected={tab === value} key={value}
        className={`studio__mode-tab${tab === value ? ' studio__mode-tab--active' : ''}`}
        onClick={() => setTab(value)}>{value === 'client' ? 'Cliente HTTP' : 'Resultados'}</button>)}
    </div>
      {tab === 'client' ? <button type="button" className="btn-secondary" disabled={disabled} onClick={explore}>executar servidor e testar rotas</button> : null}
    </div>
    {error && tab === 'client' ? <p className="status-message status-error" role="alert">{error}</p> : null}
    <div className="workspace__insights-body http-workbench__body">
      {tab === 'client' ? <div role="tabpanel" id="http-panel-client" aria-labelledby="http-tab-client" className="http-client">
        <p>Escreva em <strong>main.py</strong> no editor. Este cliente inicia seu servidor Python e envia pedidos às rotas que você implementou. A resposta mostra o que seu código fez; explorar não registra conclusão.</p>
        <p className="muted">Método é a ação; Path é a rota; Query são parâmetros de consulta; Body são os dados enviados. O lote executa em ordem e compartilha estado. Cada envio inicia um servidor novo.</p>
        {challenge.sample_test_cases.length ? <button type="button" className="btn-secondary" disabled={disabled} onClick={() => {
          setRequests(initialRequests(challenge.sample_test_cases)); setSelected(0); setError(null)
        }}>Carregar exemplos do enunciado</button> : null}
        <div className="http-client__batch">
          <label>Requisição<select aria-label="Requisição" value={selected} disabled={disabled} onChange={event => setSelected(Number(event.target.value))}>
            {requests.map((request, index) => <option key={index} value={index}>{index + 1}. {request.method} {request.path}</option>)}
          </select></label>
          <button type="button" className="btn-secondary" disabled={disabled || requests.length >= 10} onClick={() => {
            setRequests(previous => [...previous, { method: 'GET', path: '/', query: '', body: '' }]); setSelected(requests.length)
          }}>Adicionar requisição</button>
          <button type="button" className="btn-secondary" disabled={disabled || requests.length === 1} onClick={() => {
            setRequests(previous => previous.filter((_, index) => index !== selected)); setSelected(Math.max(0, selected - 1))
          }}>Remover requisição</button>
        </div>
        <div className="http-client__route"><label>Método<select aria-label="Método" value={draft.method} disabled={disabled} onChange={event => update({ method: event.target.value })}>
          {['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'].map(method => <option key={method}>{method}</option>)}
        </select></label><label>Path<input value={draft.path} disabled={disabled} placeholder="/items/1" onChange={event => update({ path: event.target.value })}/></label></div>
        <label>Query<input value={draft.query} disabled={disabled} placeholder="limit=10&offset=0" onChange={event => update({ query: event.target.value })}/></label>
        <label>Body JSON (opcional)<textarea aria-label="Body JSON (opcional)" rows={4} value={draft.body} disabled={disabled} spellCheck={false} placeholder={'{"text": "Olá"}'} onChange={event => update({ body: event.target.value })}/></label>
        <details><summary>Headers da requisição</summary><p>Content-Type: {draft.body.trim() ? 'application/json' : 'não enviado'}</p>
          <p className="muted">O servidor de execução define este header automaticamente para corpos JSON. Headers personalizados não são aceitos pelo contrato atual.</p></details>
      </div> : null}
      {showResults ? <div role={tab === 'results' ? 'tabpanel' : undefined} id="http-panel-results" aria-labelledby="http-tab-results" aria-live="polite">
        {execution.error ? <p className="status-message status-error" role="alert">{execution.error}</p> : null}
        {execution.pending ? <p className="status-message" role="status">Servidor em execução / aguardando o Judge...</p> : null}
        {execution.submission && execution.submission.status !== 'pending' ? <>
          <p className="eyebrow">{execution.submission.http_exploration ? 'Resultado exploratório (sem nota)' : 'Avaliação automática'}</p>
          <p className="status-message">{execution.submission.status === 'accepted'
            ? execution.submission.http_exploration ? 'As requisições foram executadas. Confira o retorno: explorar não compara respostas esperadas nem conclui a atividade. Quando estiver pronto, submeta para avaliação.' : 'Sua solução passou na avaliação. Releia o que aprendeu e siga para a próxima atividade.'
            : execution.submission.status === 'wrong_answer' ? 'O servidor respondeu, mas houve uma diferença no contrato. Compare método, rota, status e JSON no depurador; ajuste uma coisa por vez e teste novamente.'
            : execution.submission.status === 'time_limit_exceeded' ? 'A execução excedeu o tempo disponível. Revise laços e operações que podem impedir o servidor de responder; depois execute novamente.'
            : 'A execução não terminou como esperado. Leia os logs, confira a sintaxe Python e a definição de app em main.py. Corrija e teste novamente.'}</p>
          <SubmissionResult submission={execution.submission}>
            <Link className="back-link" to={`/challenges/${challenge.id}/submissions/${execution.submission.id}${new URLSearchParams(window.location.search).get('liveSessionId') ? window.location.search : ''}`}>Abrir resultado salvo →</Link>
          </SubmissionResult>
        </> : !execution.pending && !execution.error ? <p className="status-message">Comece pelo enunciado, complete main.py no editor e abra Cliente HTTP. Os exemplos já estão prontos: use executar servidor e testar rotas. Depois compare o retorno e submeta sua solução para avaliação.</p> : null}
      </div> : null}
    </div>
  </section>
}
