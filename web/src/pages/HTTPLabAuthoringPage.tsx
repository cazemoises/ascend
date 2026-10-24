import { useEffect, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { createHTTPLab, getProblemList, type CreateTestCaseInput, type ListItem, type ProblemListDetail } from '../api'
import { useAuth } from '../auth/useAuth'

function LabEditor({item}: {item: ListItem}) {
 const [code,setCode]=useState('')
 const [tests,setTests]=useState('[]')
 const [timeout,setTimeoutMs]=useState(10000)
 const [memory,setMemory]=useState(128)
 const [created,setCreated]=useState<string|null>(item.linked_challenge_id)
 const [pending,setPending]=useState(false)
 const [error,setError]=useState<string|null>(null)
 async function save(event:FormEvent<HTMLFormElement>){
  event.preventDefault();setError(null);setPending(true)
  try {
   const parsed: unknown=JSON.parse(tests)
   if(!Array.isArray(parsed)) throw new Error('Os testes devem ser um array JSON.')
   const result=await createHTTPLab(item.id,{starter_code:code,time_limit_ms:timeout,memory_limit_mb:memory,test_cases:parsed as CreateTestCaseInput[]})
   setCreated(result.challenge_id)
  }catch(cause){setError(cause instanceof Error?cause.message:'Falha ao criar laboratório')}
  finally{setPending(false)}
 }
 return <article className="panel student-card"><h2>{item.title}</h2>
 {created ? <p>Item vinculado ao judge. <Link to={`/challenges/${created}`}>Abrir exercício</Link></p> : item.assessment_revision_id ? <p>Este item já tem correção automática.</p> : <form onSubmit={save} className="student-card__section lab-form">
 <label>Código inicial — main.py<textarea required rows={12} maxLength={65536} value={code} onChange={e=>setCode(e.target.value)}/></label>
 <label>Testes HTTP (JSON)<textarea required rows={14} value={tests} onChange={e=>setTests(e.target.value)}/></label>
 <p>De 1 a 30 testes, na ordem de execução. input é uma string JSON com method, path (incluindo query) e json opcional. expected_output contém status e json; headers é opcional (content-type, content-language, cache-control, allow, content-encoding). Nomes de headers ignoram maiúsculas; valores são exatos e somente headers declarados são avaliados. Cookies e autenticação não são suportados; use null para corpo vazio. is_sample controla a visibilidade do pedido e da comparação. Coloque testes públicos antes dos ocultos: respostas posteriores a um caso oculto também são protegidas. Objetos JSON são comparados sem considerar a ordem das chaves; arrays mantêm a ordem.</p>
 <label>Tempo total por execução (ms)<input type="number" min={1000} max={30000} value={timeout} onChange={e=>setTimeoutMs(Number(e.target.value))}/></label>
 <label>Memória (MiB)<input type="number" min={64} max={512} value={memory} onChange={e=>setMemory(Number(e.target.value))}/></label>
 <p>A configuração fica imutável após a criação. Para outra versão, crie outro item. A publicação da lista continua manual.</p>
 <button className="challenge-submit" disabled={pending}>{pending?'criando...':'criar laboratório HTTP'}</button>
 {error?<p role="alert" className="status-error">{error}</p>:null}
 </form>}
 </article>
}
export function HTTPLabAuthoringPage(){
 const {id}=useParams<{id:string}>();const {user}=useAuth()
 const [list,setList]=useState<ProblemListDetail|null>(null);const [error,setError]=useState<string|null>(null)
 useEffect(()=>{let active=true;if(id&&user)void getProblemList(id).then(data=>{if(data.teacher_id!==user.id)throw new Error('Somente o mentor proprietário pode criar laboratórios.');if(active)setList(data)}).catch(cause=>{if(active)setError(cause instanceof Error?cause.message:'Falha ao carregar lista')});return()=>{active=false}},[id,user])
 return <main className="page-shell"><Link to={`/listas/${id}`}>← lista</Link><h1>Laboratórios práticos · APIs HTTP</h1><p>Adicione um laboratório a um item sem correção automática. O enunciado é o texto do item. O aluno edita main.py e exporta app = FastAPI().</p>
 {error?<p role="alert">{error}</p>:null}{!list&&!error?<p>Carregando...</p>:null}{list?.items.length===0?<p>Adicione itens na edição da lista.</p>:null}{list?.items.map(item=><LabEditor key={item.id} item={item}/>)}
 </main>
}
