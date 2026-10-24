import type { HTTPRequest, SampleTestCase } from '../api'
import { requestDraft, requestText } from '../lib/httpClient'

export function HTTPExamples({ samples }: { samples: SampleTestCase[] }) {
  if (!samples.length) return <p className="status-message">Sem exemplos públicos. Use o contrato do enunciado para montar uma requisição no cliente HTTP.</p>
  return <><h2 className="section-title">Exemplos de funcionamento</h2><p className="muted">O cliente envia um pedido à rota do seu código Python. Compare a resposta com o resultado esperado abaixo.</p>{samples.map((sample, index) => {
    try {
      const request = JSON.parse(sample.input) as HTTPRequest
      const response = JSON.parse(sample.expected_output) as { status: number; headers?: Record<string, string>; json: unknown }
      const draft = requestDraft(request)
      return <article className="sample" key={index} aria-label={`Exemplo ${index + 1}`}>
        <div className="sample__label">Exemplo {index + 1} · <strong>{request.method}</strong> <code>{draft.path}</code></div>
        <div className="sample__io"><div><span>Pedido do cliente</span>
          <p>Parâmetros de consulta: <code>{draft.query || 'nenhum'}</code></p>
          <p>Headers: {draft.body ? 'Content-Type: application/json' : 'nenhum exigido neste pedido'}</p>
          <p>Body (dados enviados):</p><pre>{draft.body || 'Sem corpo — os dados estão na rota ou na consulta.'}</pre></div>
          <div><span>Resposta esperada · HTTP {response.status}</span>
            <p>{response.status < 300 ? 'O pedido foi atendido.' : 'Esta resposta de erro faz parte do comportamento esperado.'}</p>
            <p>Headers esperados: {Object.entries(response.headers ?? {}).map(([name, value]) => `${name}: ${value}`).join('; ') || 'sem exigência adicional'}</p>
            <pre>{JSON.stringify(response.json, null, 2)}</pre></div></div>
        <details><summary>Ver representação técnica completa</summary><pre>{requestText(request)}</pre><pre>{sample.expected_output}</pre></details>
      </article>
    } catch { return <p className="status-message" key={index}>Exemplo indisponível. Consulte o contrato no enunciado.</p> }
  })}</>
}
