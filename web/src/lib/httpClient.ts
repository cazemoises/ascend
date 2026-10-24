import type { HTTPRequest, SampleTestCase } from '../api'

export interface RequestDraft { method: string; path: string; query: string; body: string }
export function requestDraft(request: HTTPRequest): RequestDraft {
  const index = request.path.indexOf('?')
  return { method: request.method, path: index < 0 ? request.path : request.path.slice(0, index),
    query: index < 0 ? '' : request.path.slice(index + 1),
    body: 'json' in request ? JSON.stringify(request.json, null, 2) : '' }
}
export function initialRequests(samples: SampleTestCase[]): RequestDraft[] {
  const requests = samples.slice(0, 10).flatMap(sample => {
    try {
      const request = JSON.parse(sample.input) as HTTPRequest
      return typeof request.method === 'string' && typeof request.path === 'string' ? [requestDraft(request)] : []
    } catch { return [] }
  })
  return requests.length ? requests : [{ method: 'GET', path: '/', query: '', body: '' }]
}
export function serializeRequests(drafts: RequestDraft[]): HTTPRequest[] {
  if (!drafts.length || drafts.length > 10) throw new Error('Use de 1 a 10 requisições por lote.')
  return drafts.map((draft, index) => {
    const path = draft.path.trim()
    if (!path.startsWith('/') || path.startsWith('//') || /[?\s\\#]/.test(path)) {
      throw new Error(`Requisição ${index + 1}: informe um path relativo, como /items/1. Use o campo Query para parâmetros.`)
    }
    let json: unknown
    if (draft.body.trim()) {
      try { json = JSON.parse(draft.body) } catch { throw new Error(`Requisição ${index + 1}: corpo JSON inválido.`) }
    }
    const query = draft.query.trim().replace(/^\?/, '')
    if (/[\r\n#]/.test(query)) throw new Error(`Requisição ${index + 1}: query inválida.`)
    return { method: draft.method, path: path + (query ? `?${new URLSearchParams(query).toString()}` : ''),
      ...(draft.body.trim() ? { json } : {}) }
  })
}
export function requestText(request: HTTPRequest): string {
  return `${request.method} ${request.path}${'json' in request ? `\nContent-Type: application/json\n\n${JSON.stringify(request.json, null, 2)}` : ''}`
}

// Old authored statements include Judge request envelopes. Present those as
// HTTP examples without modifying stored content or hiding other code blocks.
export function httpDescription(description: string): string {
  return description.replace(/Cole este lote no campo de requisições e use \*\*Explorar\*\* \(sem nota\):/g,
    'Use o cliente HTTP ao lado do editor para explorar estas requisições (sem nota):')
    .replace(/```json\s*\n([\s\S]*?)```/g, (block: string, content: string) => {
      try {
        const requests: unknown = JSON.parse(content)
        if (!Array.isArray(requests) || !requests.length || !requests.every((r: unknown) =>
          typeof r === 'object' && r !== null && 'method' in r && typeof r.method === 'string' && 'path' in r && typeof r.path === 'string')) return block
        return `\`\`\`http\n${(requests as HTTPRequest[]).map(requestText).join('\n\n────────\n\n')}\n\`\`\``
      } catch { return block }
    })
}
