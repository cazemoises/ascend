import Editor from '@monaco-editor/react'
import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import { createSubmission, getLastSubmission, getSubmission, type Challenge, type HTTPRequest, type Submission } from '../api'
import { HTTPTestDebugger } from '../components/HTTPTestDebugger'
import { VerdictBadge } from '../components/VerdictBadge'
import { useAuth } from '../auth/useAuth'

export function HTTPLabWorkspace({challenge}:{challenge:Challenge}){
 const {user,isTeacher}=useAuth()
 const [code,setCode]=useState(challenge.starter_code??'')
 const [requests,setRequests]=useState(JSON.stringify([{method:'GET',path:'/hello/Ana?greeting=Oi'}],null,2))
 const [submission,setSubmission]=useState<Submission|null>(null)
 const [pending,setPending]=useState(false)
 const [mode,setMode]=useState<'explore'|'grade'>('explore')
 const [error,setError]=useState<string|null>(null)
 const generation=useRef(0)
 useEffect(()=>{let active=true;const guard=generation;void getLastSubmission(challenge.id).then(last=>{if(active)setCode(last.source_code)}).catch(()=>{});return()=>{active=false;guard.current++}},[challenge.id,user?.id])
 async function run(explore:boolean){
  const current=++generation.current
  setPending(true);setError(null);setSubmission(null);setMode(explore?'explore':'grade')
  try{
   let batch:HTTPRequest[]|undefined
   if(explore){const parsed:unknown=JSON.parse(requests);if(!Array.isArray(parsed)||parsed.length<1||parsed.length>10)throw new Error('Informe de 1 a 10 requisições em um array JSON.');batch=parsed as HTTPRequest[]}
   const created=await createSubmission(challenge.id,{language:'fastapi',source_code:code,...(batch?{http_requests:batch}:{})})
   while(current===generation.current){
    const result=await getSubmission(created.submission_id)
    if(current!==generation.current)return
    setSubmission(result)
    if(result.status!=='pending')break
    await new Promise(resolve=>window.setTimeout(resolve,1000))
   }
  }catch(cause){if(current===generation.current)setError(cause instanceof Error?cause.message:'Falha na execução')}
  finally{if(current===generation.current)setPending(false)}
 }
 return <main className="page-shell"><Link to="/listas">← listas</Link><p className="eyebrow">Laboratório prático · FastAPI</p><h1>{challenge.title}</h1>
 <ReactMarkdown>{challenge.description}</ReactMarkdown>
 <p>{challenge.time_limit_ms} ms por lote · {challenge.memory_limit_mb} MiB · main.py exporta app. Cada execução inicia um ambiente novo; requisições do mesmo lote compartilham o servidor.</p>
 <section className="lab-workspace"><div><h2>main.py</h2><Editor height="460px" language="python" value={code} onChange={value=>setCode(value??'')} options={{readOnly:pending,minimap:{enabled:false},automaticLayout:true}}/></div>
 <div className="lab-form"><h2>Exploração HTTP</h2><label>Requisições (array JSON)<textarea rows={12} value={requests} onChange={e=>setRequests(e.target.value)} disabled={pending}/></label><p>Use method, path com parâmetros e json opcional. Explorar mostra as respostas sem avaliar nem concluir o exercício. O servidor encerra ao terminar o lote.</p>
 <button className="btn-secondary" disabled={pending} onClick={()=>void run(true)}>executar servidor e testar rotas</button>
 <button className="challenge-submit" disabled={pending} onClick={()=>void run(false)}>{isTeacher?'testar avaliação como mentor':'submeter para avaliação'}</button>
 <p>{isTeacher?'Execuções do mentor não contam para o progresso do aluno.':'A submissão avaliada usa os testes do mentor e conta para o progresso.'}</p>
 </div></section>
 {challenge.sample_test_cases.length>0?<details><summary>Testes de exemplo</summary>{challenge.sample_test_cases.map((tc,i)=><div key={i}><pre>{tc.input}</pre><pre>{tc.expected_output}</pre></div>)}</details>:null}
 {error?<p role="alert" className="status-error">{error}</p>:null}
 {pending?<p role="status">Servidor em execução / aguardando o Judge...</p>:null}
 {submission?<section aria-live="polite"><h2>{mode==='explore'?'Resultado exploratório (sem nota)':'Avaliação automática'}</h2><VerdictBadge status={submission.status}/><p>{submission.passed_count??0} / {submission.total_test_cases??0} {mode==='explore'?'requisições executadas':'testes aprovados'}</p>
 <HTTPTestDebugger key={submission.id} submission={submission}/>
 <Link to={`/challenges/${challenge.id}/submissions/${submission.id}`}>Abrir resultado salvo</Link>
 </section>:null}
 </main>
}
