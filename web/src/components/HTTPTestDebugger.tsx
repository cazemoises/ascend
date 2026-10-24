import { useState } from 'react'
import type { Submission } from '../api'
import { httpDebuggerView } from '../lib/httpDebugger'

const labels: Record<string, string> = {
  status: 'Status HTTP', header: 'Header HTTP', type: 'Tipo diferente',
  missing: 'Campo ou elemento ausente', extra: 'Campo ou elemento extra',
  value: 'Valor diferente', invalid_json: 'Corpo não é JSON',
}

export function HTTPTestDebugger({ submission }: { submission: Submission }) {
  const [technical, setTechnical] = useState(false)
  const [selected, setSelected] = useState(0)
  const view = httpDebuggerView(submission)
  if (!view) return null
  if (view.kind === 'unavailable') return <section className="http-debugger panel"><h2>Visual Test Debugger</h2><p>{submission.status === 'accepted' ? 'Avaliação concluída. Nenhuma divergência pública disponível.' : 'Comparação indisponível: casos ocultos, respostas que dependem deles e resultados antigos não exibem dados de avaliação.'}</p><p>Use a exploração HTTP ou um teste público para investigar sua solução.</p></section>
  const differences = view.comparison?.differences ?? []
  const current = differences[selected] ?? differences[0]
  return <section className="http-debugger panel" aria-label="Visual Test Debugger">
    <h2>Visual Test Debugger</h2>
    <p>{view.kind === 'response' ? 'Resposta pública e diagnósticos da execução. Não há comparação detalhada disponível.' : view.kind === 'exploration' ? 'Exploração sem nota: apenas respostas às requisições que você enviou. Não há gabarito para comparar.' : 'Comparação do primeiro teste público com falha. Selecione uma diferença para entender o que mudou.'}</p>
    <div className="http-debugger__modes" aria-label="Modo de visualização">
      <button type="button" aria-pressed={!technical} onClick={() => setTechnical(false)}>Visão simples</button>
      <button type="button" aria-pressed={technical} onClick={() => setTechnical(true)}>Visão técnica</button>
    </div>
    {view.kind === 'comparison' ? <>
      <p>Objetos ignoram a ordem das chaves; arrays comparam posição por posição. Números equivalentes têm o mesmo valor. Headers declarados pelo mentor são obrigatórios; headers extras não alteram a nota.</p>
      <ol className="http-debugger__differences" aria-label="Divergências">
        {differences.map((difference, index) => <li key={`${difference.path}-${difference.kind}`}><button type="button" aria-pressed={current === difference} onClick={() => setSelected(index)}><strong>{labels[difference.kind] ?? 'Diferença'}</strong> <code>{difference.path}</code></button></li>)}
      </ol>
      {view.comparison?.truncated ? <p role="status">Diagnóstico limitado a 100 diferenças e 32 níveis. Consulte o JSON bruto para os demais detalhes públicos.</p> : null}
      {current ? <div aria-live="polite"><h3>Caminho: <code>{current.path}</code></h3><p>{current.explanation}</p>
        <div className="http-debugger__columns">
          <div><h3>Esperado</h3><pre className="http-debugger__difference">{current.expected ?? 'Ausente'}</pre></div>
          <div><h3>Recebido</h3><pre className="http-debugger__difference">{current.actual ?? 'Ausente'}</pre></div>
        </div>
      </div> : <p>Nenhuma diferença detalhada disponível nesta resposta.</p>}
    </> : <p>Confira as respostas abaixo. A visão técnica inclui o lote bruto e os logs da execução.</p>}
    {view.actual !== null ? <HTTPResponses raw={view.actual} /> : null}
    {technical ? <>
      {view.kind === 'comparison' && submission.failed_input ? <><h3>Requisição pública</h3><pre>{submission.failed_input}</pre></> : null}
      <div className="http-debugger__columns">
        {view.expected !== null ? <div><h3>Esperado · JSON bruto</h3><pre>{view.expected}</pre></div> : null}
        <div><h3>Recebido · JSON bruto</h3><pre>{view.actual ?? 'Sem resposta HTTP disponível.'}</pre></div>
      </div>
      <h3>Logs e diagnósticos</h3><pre>{view.logs ?? 'Logs indisponíveis. Lotes com testes ocultos têm seus logs protegidos.'}</pre>
    </> : null}
  </section>
}

// Only receives data already authorized by httpDebuggerView. Never parse the
// submission directly: hidden or state-tainted snapshots must fail closed.
function HTTPResponses({ raw }: { raw: string }) {
  let parsed: unknown
  try { parsed = JSON.parse(raw) } catch { return null }
  const responses: unknown[] = Array.isArray(parsed) ? parsed : [parsed]
  return <div aria-label="Respostas HTTP">{responses.map((response, index) => {
    if (typeof response !== 'object' || response === null || !('status' in response) || typeof response.status !== 'number') return null
    const body = 'raw_body' in response && typeof response.raw_body === 'string' ? response.raw_body :
      'json' in response ? JSON.stringify(response.json, null, 2) : 'Sem corpo'
    const headers = 'headers' in response && typeof response.headers === 'object' && response.headers !== null ? response.headers : {}
    return <section className="result-panel" key={index}>
      <h3 className="result-panel__header">Resposta {index + 1} · HTTP {response.status}</h3>
      <div className="result-panel__body"><details><summary>Headers da resposta</summary><pre>{Object.entries(headers).map(([name, value]) => `${name}: ${String(value)}`).join('\n') || 'Sem headers disponíveis'}</pre></details>
        <pre>{body}</pre></div>
    </section>
  })}</div>
}
