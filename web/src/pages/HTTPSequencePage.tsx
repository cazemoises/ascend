import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import { getHTTPSequence, importProblemList, listProblemLists, type ImportProblemListInput, type ProblemList } from '../api'
import { useAuth } from '../auth/useAuth'

export function HTTPSequencePage() {
  const navigate = useNavigate()
  const { user } = useAuth()
  const [sequence, setSequence] = useState<ImportProblemListInput | null>(null)
  const [source, setSource] = useState('')
  const [reviewed, setReviewed] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [existing, setExisting] = useState<ProblemList[]>([])
  const [uncertain, setUncertain] = useState(false)
  useEffect(() => {
    let active = true
    void Promise.all([getHTTPSequence(), listProblemLists()]).then(([data, lists]) => {
      if (!active) return
      setSequence(data)
      setSource(JSON.stringify(data, null, 2))
      setExisting(lists.filter(list => list.teacher_id === user?.id && list.title === data.title))
    }).catch(cause => { if (active) setError(cause instanceof Error ? cause.message : 'Falha ao carregar sequência') })
    return () => { active = false }
  }, [user?.id])

  function preview() {
    setError(null)
    setReviewed(false)
    try {
      const parsed = JSON.parse(source) as ImportProblemListInput
      if (!parsed || typeof parsed.title !== 'string' || !parsed.title.trim() || !Array.isArray(parsed.items) || !parsed.items.length || parsed.items.length > 30 || parsed.items.some(item => !item || typeof item.title !== 'string' || typeof item.body !== 'string' || !item.http_lab || typeof item.http_lab.starter_code !== 'string' || !Array.isArray(item.http_lab.test_cases) || item.http_lab.test_cases.some(test => !test || typeof test.input !== 'string' || typeof test.expected_output !== 'string' || typeof test.is_sample !== 'boolean'))) {
        throw new Error('Informe título e de 1 a 30 itens com enunciado, http_lab, starter_code e test_cases. A API valida os contratos HTTP ao cadastrar.')
      }
      setSequence(parsed)
    } catch (cause) {
      setSequence(null)
      setError(cause instanceof Error ? cause.message : 'JSON inválido')
    }
  }

  async function createDraft() {
    if (!sequence || !reviewed || pending || uncertain) return
    setPending(true)
    setError(null)
    try {
      const created = await importProblemList(sequence)
      navigate(`/listas/${created.id}`)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Falha ao criar sequência')
      // The server may have committed before a connection failed. Never retry
      // automatically or invite a second import without checking the list index.
      setUncertain(true)
    } finally { setPending(false) }
  }

  return <main className="page-shell">
    <Link to="/listas">← listas</Link>
    <h1>Criação assistida · Laboratórios FastAPI</h1>
    <p>Esta sequência foi preparada por IA para revisão do mentor. A aprovação abaixo cadastra uma nova lista em rascunho, com enunciados, código inicial e testes. A publicação é uma ação separada na página da lista. A correção das submissões usa testes determinísticos, sem IA.</p>
    {existing.length > 0 ? <aside className="panel"><p>Você já tem uma lista com o título desta sequência. Confira antes de criar outra:</p>{existing.map(list => <p key={list.id}><Link to={`/listas/${list.id}`}>{list.title} · {list.published ? 'publicada' : 'rascunho'}</Link></p>)}</aside> : null}
    {error ? <p role="alert" className="status-error">{error}</p> : null}
    {!source && !error ? <p>Carregando sequência...</p> : null}
    {source ? <>
      <details className="panel"><summary>Editar ou reutilizar o pacote JSON</summary>
        <p>Para outra atividade assistida, substitua este pacote por um JSON produzido pela IA no mesmo formato. Revise os contratos e os testes antes de aprovar. Nenhuma chamada a IA ocorre durante a correção.</p>
        <label>Pacote de atividades<textarea className="input-code input-code--tall" rows={20} value={source} disabled={pending || uncertain} onChange={event => { setSource(event.target.value); setSequence(null); setReviewed(false) }} /></label>
        <button type="button" className="btn-secondary" disabled={pending || uncertain} onClick={preview}>Atualizar revisão</button>
      </details>
      {sequence ? <>
        <h2>{sequence.title}</h2><p>{sequence.description}</p>
        {sequence.items.map((item, index) => <article className="panel student-card" key={index}>
          <h2>{item.title}</h2><ReactMarkdown>{item.body}</ReactMarkdown>
          <details><summary>Revisar código inicial e testes públicos/ocultos</summary>
            <h3>main.py</h3><pre style={{ overflowX: 'auto' }}>{item.http_lab?.starter_code}</pre>
            <p>Tempo: {item.http_lab?.time_limit_ms} ms · Memória: {item.http_lab?.memory_limit_mb} MiB. A configuração fica imutável após o cadastro; revise agora.</p>
            {item.http_lab?.test_cases.map((test, testIndex) => <div key={testIndex}>
              <h3>Teste {testIndex + 1} · {test.is_sample ? 'público' : 'oculto'}</h3>
              <p>Requisição</p><pre style={{ overflowX: 'auto' }}>{test.input}</pre>
              <p>Resposta esperada</p><pre style={{ overflowX: 'auto' }}>{test.expected_output}</pre>
            </div>)}
          </details>
        </article>)}
        <label><input type="checkbox" checked={reviewed} disabled={pending || uncertain} onChange={event => setReviewed(event.target.checked)} /> Revisei enunciados, código e testes e aprovo o cadastro em uma nova lista em rascunho.</label>
        <p><button className="challenge-submit" type="button" onClick={() => void createDraft()} disabled={!reviewed || pending || uncertain}>{pending ? 'cadastrando...' : 'Aprovar e cadastrar sequência em rascunho'}</button></p>
      </> : <p>Atualize a revisão depois de editar o pacote.</p>}
    </> : null}
    {uncertain ? <p role="alert">O cadastro não será repetido automaticamente. <Link to="/listas">Confira suas listas</Link> antes de tentar uma nova importação. Se a lista foi criada, todos os laboratórios foram cadastrados juntos; se a API rejeitou o pacote, nenhum item foi salvo.</p> : null}
  </main>
}
