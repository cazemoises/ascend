import { chromium } from 'playwright'
import { readFile,writeFile } from 'node:fs/promises'
import assert from 'node:assert/strict'
const base=process.env.ASCEND_HTTP_BROWSER_BASE
if (!base || !['localhost','127.0.0.1'].includes(new URL(base).hostname)) throw new Error('Set ASCEND_HTTP_BROWSER_BASE to an isolated local fixture')
const output=process.env.ASCEND_HTTP_BROWSER_OUTPUT ?? '/tmp/ascend-http-browser'
await (await import('node:fs/promises')).mkdir(output,{recursive:true})
const pack=JSON.parse(await readFile(new URL('../../api/internal/handler/activity_templates/fastapi-v1.json',import.meta.url),'utf8'))
const browser=await chromium.launch({headless:true})
const page=await browser.newPage({viewport:{width:1440,height:1000},permissions:['clipboard-read','clipboard-write']})
page.setDefaultTimeout(15000)
const errors=[]
page.on('pageerror',e=>errors.push(e.message))
async function editCode(code) {
 await page.waitForTimeout(300)
 await page.locator('.monaco-editor .view-lines').click()
 await page.keyboard.press('ControlOrMeta+A')
 await page.evaluate(text=>navigator.clipboard.writeText(text),code)
 await page.keyboard.press('ControlOrMeta+V')
 await page.waitForTimeout(300) // Native clipboard paste is asynchronous in Chromium.
 await page.keyboard.press('ControlOrMeta+Home')
 try { await page.waitForFunction(prefix=>document.querySelector('.monaco-editor .view-lines')?.textContent?.replace(/\s/g,'').includes(prefix),code.replace(/\s/g,'').slice(0,160)) } catch(error) { console.log('Expected editor prefix',code.replace(/\s/g,'').slice(0,160),'actual',await page.locator('.monaco-editor .view-lines').textContent()); await page.screenshot({path:output+'/editor-failure.png'}); throw error }
 await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))
}
async function submit(name, code) {
 const [request]=await Promise.all([
  page.waitForRequest(request=>request.method()==='POST' && /\/submissions$/.test(new URL(request.url()).pathname)),
  page.getByRole('button',{name,exact:true}).click(),
 ])
 assert.equal(request.postDataJSON().source_code,code,'Judge receives the code edited in Monaco')
}
try {
 await page.goto(base+'/listas/nova')
 const existing=await page.request.get(new URL('/api/v1/lists',base).href).then(r=>r.json())
 assert.equal(existing.filter(list=>list.title===pack.title).length,0,'Use a fresh disposable fixture; do not duplicate the activity package')
 await page.getByRole('tab',{name:'Importar JSON'}).click()
 await page.getByLabel('JSON',{exact:true}).fill(JSON.stringify(pack))
 await page.getByRole('button',{name:'Pré-visualizar'}).click()
 assert.equal(await page.getByText('Laboratório HTTP com Judge',{exact:false}).count(),6)
 await page.getByRole('button',{name:'criar lista',exact:true}).click()
 await page.waitForURL(/\/listas\/[0-9a-f-]+$/)
 const listURL=page.url()
 await page.getByRole('heading',{name:pack.title,exact:true}).waitFor()
 await page.getByRole('button',{name:'publicar',exact:true}).click()
 await page.getByText('RASCUNHO',{exact:true}).waitFor({state:'hidden'})
 console.log('PASS mentor: normal JSON import, six persisted activities, publish '+listURL)
 await writeFile(output+'/list.json',JSON.stringify({url:listURL,title:pack.title}))
 await page.getByRole('button',{name:'ver como aluno',exact:true}).click()
 // Only this isolated localhost fixture accepts simulated Pangolin identities.
 await page.setExtraHTTPHeaders({'Remote-Email':'http-browser-student@example.test'})
 await page.goto(base+'/listas')
 const student=await page.evaluate(()=>fetch('/api/v1/auth/me').then(response=>response.json()))
 assert.equal(student.real_role,'student')
 assert.equal(student.effective_role,'student')
 await page.locator('article').filter({has:page.getByRole('heading',{name:pack.title,exact:true})}).getByRole('link',{name:'ver lista'}).click()
 await page.waitForURL(listURL)
 await page.getByRole('heading',{name:pack.title,exact:true}).waitFor()
 const links=await page.locator('a[href*="/challenges/"]').evaluateAll(elements=>elements.map(e=>e.getAttribute('href')))
 assert.equal(new Set(links).size,6)
 for (let i=0;i<6;i++) {
  await page.goto(new URL(links[i],base).href)
  await page.getByRole('heading',{name:pack.items[i].title,exact:true}).waitFor()
  await page.locator('.monaco-editor textarea').waitFor()
  const requests=JSON.parse(await page.getByLabel('Requisições (array JSON)').inputValue())
  assert.deepEqual(requests,pack.items[i].http_lab.test_cases.filter(t=>t.is_sample).slice(0,10).map(t=>JSON.parse(t.input)))
 }
 console.log('PASS student: six links, rendered statements, Monaco and activity-specific requests')
 const references=await readFile(new URL('../../judge/internal/worker/http_sequence_integration_test.go',import.meta.url),'utf8')
 const solutions=[...references.split('solutions := []string{')[1].split('\n\t}')[0].matchAll(/`([^`]+)`/g)].map(match=>match[1])
 assert.equal(solutions.length,6)
 for (let index=0;index<6;index++) {
 await page.goto(new URL(links[index],base).href)
 await page.locator('.monaco-editor textarea').waitFor()
 await editCode(pack.items[index].http_lab.starter_code)
 await submit('executar servidor e testar rotas',pack.items[index].http_lab.starter_code)
 await page.getByText('Exploração sem nota:',{exact:false}).waitFor({timeout:45000})
 await page.getByRole('button',{name:'Visão técnica'}).click()
 await page.getByRole('heading',{name:'Logs e diagnósticos'}).waitFor()
 await page.screenshot({path:output+`/exploration-${index+1}.png`})
 console.log('PASS exploration: real Judge, HTTP response and logs')
 await submit('submeter para avaliação',pack.items[index].http_lab.starter_code)
 await page.getByRole('heading',{name:'Caminho:',exact:false}).waitFor({timeout:45000})
 await page.getByRole('button',{name:'Visão técnica'}).click()
 await page.getByRole('heading',{name:'Requisição pública',exact:true}).waitFor()
 await page.screenshot({path:output+`/wrong-answer-${index+1}.png`})
 console.log('PASS wrong answer: public feedback, Visual Test Debugger and raw comparison')
 await editCode(solutions[index])
 await submit('submeter para avaliação',solutions[index])
 await page.getByText('Aceito',{exact:true}).waitFor({timeout:45000})
 await page.screenshot({path:output+`/accepted-${index+1}.png`})

 console.log(`PASS activity ${index+1}: exploration, wrong answer, debugger, Monaco correction and acceptance`)
 }
 await page.goto(listURL)
 await page.getByRole('heading',{name:pack.title,exact:true}).waitFor()
 assert.equal(await page.getByText('CONCLUÍDO',{exact:true}).count(),6)
 console.log('PASS correct answer: edit in Monaco, graded acceptance and list progress')
 assert.deepEqual(errors,[])
} finally { await browser.close() }
