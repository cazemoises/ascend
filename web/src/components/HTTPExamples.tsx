import type { HTTPRequest, SampleTestCase } from '../api'
import { requestText } from '../lib/httpClient'

export function HTTPExamples({ samples }: { samples: SampleTestCase[] }) {
  if (!samples.length) return null
  return <><h2 className="section-title">Exemplos</h2>{samples.map((sample, index) => {
    try {
      const request = JSON.parse(sample.input) as HTTPRequest
      const response = JSON.parse(sample.expected_output) as { status: number; headers?: Record<string, string>; json: unknown }
      return <div className="sample" key={index}>
        <div className="sample__label">Exemplo {index + 1}</div>
        <div className="sample__io"><div><span>Requisição</span><pre>{requestText(request)}</pre></div>
          <div><span>Resposta esperada · HTTP {response.status}</span>
            <pre>{Object.entries(response.headers ?? {}).map(([name, value]) => `${name}: ${value}\n`)}{'\n'}{JSON.stringify(response.json, null, 2)}</pre></div></div>
      </div>
    } catch { return <p className="status-message" key={index}>Exemplo indisponível.</p> }
  })}</>
}
