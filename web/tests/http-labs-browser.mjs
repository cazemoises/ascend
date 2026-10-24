import { chromium } from 'playwright'
import { readFile,writeFile } from 'node:fs/promises'
import assert from 'node:assert/strict'
const base=process.env.ASCEND_HTTP_BROWSER_BASE
if (!base || !['localhost','127.0.0.1'].includes(new URL(base).hostname)) throw new Error('Set ASCEND_HTTP_BROWSER_BASE to an isolated local fixture')
const phase=process.env.ASCEND_HTTP_BROWSER_PHASE ?? 'all'
const output=process.env.ASCEND_HTTP_BROWSER_OUTPUT ?? '/tmp/ascend-http-browser'
await (await import('node:fs/promises')).mkdir(output,{recursive:true})
const pack=JSON.parse(await readFile(new URL('../../api/internal/handler/activity_templates/fastapi-pedagogy-v2.json',import.meta.url),'utf8'))
const browser=process.env.ASCEND_HTTP_BROWSER_WS
 ? await chromium.connect(process.env.ASCEND_HTTP_BROWSER_WS,{exposeNetwork:'<loopback>'})
 : await chromium.launch({headless:true})
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
 try { await page.waitForFunction(prefix=>Array.from(document.querySelectorAll('.monaco-editor .view-line')).sort((a,b)=>a.getBoundingClientRect().top-b.getBoundingClientRect().top).map(line=>line.textContent).join('').replace(/\s/g,'').includes(prefix),code.replace(/\s/g,'').slice(0,160)) } catch(error) { console.log('Expected editor prefix',code.replace(/\s/g,'').slice(0,160),'actual',await page.locator('.monaco-editor .view-lines').textContent()); await page.screenshot({path:output+'/editor-failure.png'}); throw error }
 await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))
}
async function submit(name, code) {
 const [request]=await Promise.all([
  page.waitForRequest(request=>request.method()==='POST' && /\/submissions$/.test(new URL(request.url()).pathname)),
  page.getByRole('button',{name,exact:true}).click(),
 ])
 assert.equal(request.postDataJSON().source_code,code,'Judge receives the code edited in Monaco')
 return request.postDataJSON()
}
try {
 let links, listURL, traditional
 if(phase==='all' || phase==='setup') {
 // All records below belong to this explicitly disposable localhost database.
 const traditionalResponse=await page.request.post(new URL('/api/v1/challenges',base).href,{data:{
  slug:'workspace-browser-traditional',title:'Workspace · desafio tradicional',difficulty:'easy',
  description:'## Objetivo\nImprima `ok`.\n\n```python\nprint("ok")\n```',
  time_limit_ms:10000,memory_limit_mb:128,starter_code:'print("ok")\n',
 }})
 assert.equal(traditionalResponse.status(),201)
 traditional=await traditionalResponse.json()
 const sampleResponse=await page.request.post(new URL(`/api/v1/challenges/${traditional.id}/test-cases`,base).href,{data:{input:'',expected_output:'ok',is_sample:true}})
 assert.equal(sampleResponse.status(),201)
 await page.goto(base+'/listas/nova')
 const existing=await page.request.get(new URL('/api/v1/lists',base).href).then(r=>r.json())
 assert.equal(existing.filter(list=>list.title===pack.title).length,0,'Use a fresh disposable fixture; do not duplicate the activity package')
 await page.getByRole('tab',{name:'Importar JSON'}).click()
 await page.getByLabel('JSON',{exact:true}).fill(JSON.stringify(pack))
 await page.getByRole('button',{name:'Pré-visualizar'}).click()
 assert.equal(await page.getByText('Laboratório HTTP com Judge',{exact:false}).count(),6)
 await page.getByRole('button',{name:'criar lista',exact:true}).click()
 await page.waitForURL(/\/listas\/[0-9a-f-]+$/)
 listURL=page.url()
 await page.getByRole('heading',{name:pack.title,exact:true,level:1}).waitFor()
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
 await page.getByRole('heading',{name:pack.title,exact:true,level:1}).waitFor()
 await page.getByRole('link',{name:'resolver desafio:',exact:false}).nth(5).waitFor()
 links=await page.locator('a[href*="/challenges/"]').evaluateAll(elements=>elements.map(e=>e.getAttribute('href')))
 assert.equal(new Set(links).size,6)
 for (let i=0;i<6;i++) {
  await page.goto(new URL(links[i],base).href)
  await page.getByRole('heading',{name:pack.items[i].title,exact:true}).waitFor()
  await page.locator('.monaco-editor textarea').waitFor()
  for (const name of ['Contexto real','O problema','Sua missão','Exemplo de funcionamento','Por onde começar','Contrato HTTP','Como saber que concluiu']) {
   await page.getByRole('heading',{name,exact:true}).waitFor()
  }
  await page.getByText('Ver representação técnica completa',{exact:true}).first().click()
  await page.getByRole('button',{name:'Carregar exemplos do enunciado',exact:true}).click()
  const requests=pack.items[i].http_lab.test_cases.filter(t=>t.is_sample).slice(0,10).map(t=>JSON.parse(t.input))
  assert.equal(await page.getByLabel('Requisição',{exact:true}).locator('option').count(),requests.length)
  assert.equal(await page.getByLabel('Método',{exact:true}).inputValue(),requests[0].method)
  assert.equal(await page.getByLabel('Path',{exact:true}).inputValue(),requests[0].path.split('?')[0])
  assert.equal(await page.getByLabel('Requisições (array JSON)').count(),0)
 }
 console.log('PASS student: six links, rendered statements, Monaco and activity-specific requests')
 // Compare the actual rendered application at exactly the same viewport sizes.
 const audit=[]
 for(const viewport of [{width:1440,height:1000},{width:390,height:844}]) {
  await page.setViewportSize(viewport)
  for(const [kind,path] of [['traditional',`/challenges/${traditional.id}`],['http',links[0]]]) {
   await page.goto(new URL(kind==='traditional'?`/ascend${path}`:path,base).href)
   await page.locator('.monaco-editor textarea').waitFor()
   await page.waitForTimeout(300)
   assert.equal(await page.locator('.workspace').count(),1)
   assert.equal(await page.locator('.workspace__brief .markdown-content h2').count()>0,true)
   assert.equal(await page.locator('.monaco-editor').evaluate(el=>el.classList.contains('vs-dark')),true)
   const measurement=await page.evaluate(()=>({
    pageWidth:document.documentElement.scrollWidth,viewport:innerWidth,
    editor:document.querySelector('.editor-host').getBoundingClientRect().toJSON(),
    brief:document.querySelector('.workspace__brief').getBoundingClientRect().toJSON(),
    codeBackground:getComputedStyle(document.querySelector('.workspace__editor')).backgroundColor,
   }))
   assert.ok(measurement.pageWidth<=viewport.width,`${kind}: no horizontal page overflow`)
   assert.ok(measurement.editor.height>=160)
   audit.push({kind,viewport,...measurement})
   await page.screenshot({path:output+`/workspace-${kind}-${viewport.width}.png`,fullPage:true})
   await editCode('for')
   await page.keyboard.press('ControlOrMeta+Space')
   await page.locator('.suggest-widget.visible').waitFor()
   assert.match(await page.locator('.suggest-widget.visible').innerText(),/for/)
   await page.screenshot({path:output+`/autocomplete-${kind}-${viewport.width}.png`,fullPage:true})
   await page.keyboard.press('Escape')
   await editCode('print("ok")\n')
   await page.keyboard.press('ControlOrMeta+z')
   await page.waitForFunction(()=>!document.querySelector('.monaco-editor .view-lines')?.textContent?.includes('print'))
   await page.keyboard.press('ControlOrMeta+Shift+z')
   await page.waitForFunction(()=>document.querySelector('.monaco-editor .view-lines')?.textContent?.includes('print'))
  }
 }
 await writeFile(output+'/workspace-audit.json',JSON.stringify(audit,null,2))
 await page.setViewportSize({width:1440,height:1000})
 await page.goto(new URL(`/ascend/challenges/${traditional.id}`,base).href)
 await page.locator('.monaco-editor textarea').waitFor()
 await editCode('print("ok")\n')
 await submit('enviar solução','print("ok")\n')
 await page.waitForURL(/\/submissions\//)
 await page.locator('.submission-verdict-hero').waitFor({timeout:45000})
 assert.equal(await page.getByText('Aceito',{exact:true}).count(),1,await page.locator('.submission-verdict-hero').innerText())
 await page.getByRole('link',{name:'← voltar ao desafio',exact:true}).click()
 await page.locator('.monaco-editor textarea').waitFor()
 assert.ok((await page.locator('.monaco-editor .view-lines').innerText()).includes('print'))
 console.log('PASS comparative desktop/mobile: layout, Markdown, editor theme, snippets, keyboard editing and traditional submission/navigation')
 await writeFile(output+'/session.json',JSON.stringify({links,listURL,traditional}))
 } else {
  ;({links,listURL,traditional}=JSON.parse(await readFile(output+'/session.json','utf8')))
  await page.setExtraHTTPHeaders({'Remote-Email':'http-browser-student@example.test'})
 }
 const references=await readFile(new URL('../../judge/internal/worker/http_sequence_integration_test.go',import.meta.url),'utf8')
 const solutions=[...references.split('solutions := []string{')[1].split('\n\t}')[0].matchAll(/`([^`]+)`/g)].map(match=>match[1])
 assert.equal(solutions.length,6)
 for (let index=0;index<6;index++) {
 if(phase!=='all' && phase!==String(index+1)) continue
 await page.goto(new URL(links[index],base).href)
 await page.locator('.monaco-editor textarea').waitFor()
 await editCode(pack.items[index].http_lab.starter_code)
 await page.getByRole('tab',{name:'Cliente HTTP',exact:true}).click()
 if(index===0) {
  await page.screenshot({path:output+'/workspace-http-1440.png',fullPage:true})
  await page.setViewportSize({width:390,height:844})
  await page.getByRole('button',{name:'executar servidor e testar rotas',exact:true}).scrollIntoViewIfNeeded()
  await page.screenshot({path:output+'/workspace-http-390.png',fullPage:true})
  await page.setViewportSize({width:1440,height:1000})
  await page.getByRole('button',{name:'Adicionar requisição',exact:true}).click()
  await page.getByLabel('Método',{exact:true}).selectOption('POST')
  await page.getByLabel('Path',{exact:true}).fill('/health')
  await page.getByLabel('Query',{exact:true}).fill('probe=ready')
  await page.getByLabel('Body JSON (opcional)',{exact:true}).fill('{')
  await page.getByRole('button',{name:'executar servidor e testar rotas',exact:true}).click()
  await page.getByRole('alert').filter({hasText:'corpo JSON inválido'}).waitFor()
  await page.getByLabel('Body JSON (opcional)',{exact:true}).fill('{"probe":true}')
  const posted=await submit('executar servidor e testar rotas',pack.items[index].http_lab.starter_code)
  assert.deepEqual(posted.http_requests,[{method:'GET',path:'/health'},{method:'POST',path:'/health?probe=ready',json:{probe:true}}])
 } else {
  await submit('executar servidor e testar rotas',pack.items[index].http_lab.starter_code)
 }
 await page.getByText('Exploração sem nota:',{exact:false}).waitFor({timeout:45000})
 await page.getByRole('button',{name:'Visão técnica'}).click()
 await page.getByRole('heading',{name:'Logs e diagnósticos'}).waitFor()
 await page.screenshot({path:output+`/exploration-${index+1}.png`})
 console.log('PASS exploration: real Judge, HTTP response and logs')
 await submit('submeter para avaliação',pack.items[index].http_lab.starter_code)
 await page.getByRole('heading',{name:'Caminho:',exact:false}).waitFor({timeout:45000})
 await page.locator('.http-debugger__columns').first().scrollIntoViewIfNeeded()
 await page.screenshot({path:output+`/comparison-${index+1}.png`})
 await page.getByRole('button',{name:'Visão técnica'}).click()
 await page.getByRole('heading',{name:'Requisição pública',exact:true}).waitFor()
 await page.screenshot({path:output+`/wrong-answer-${index+1}.png`})
 if(index===0) {
  await page.setViewportSize({width:390,height:844})
  await page.getByRole('button',{name:'Visão simples',exact:true}).click()
  await page.locator('.http-debugger__differences button').first().click()
  await page.locator('.http-debugger__columns').first().scrollIntoViewIfNeeded()
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
  await page.screenshot({path:output+'/debugger-mobile.png',fullPage:true})
  await page.getByRole('button',{name:'Visão técnica',exact:true}).click()
  await page.getByRole('heading',{name:'Requisição pública',exact:true}).waitFor()
  await page.setViewportSize({width:1440,height:1000})
 }
 console.log('PASS wrong answer: public feedback, Visual Test Debugger and raw comparison')
 await editCode(solutions[index])
 await submit('submeter para avaliação',solutions[index])
 await page.locator('.submission-verdict-hero').waitFor({timeout:45000})
 assert.equal(await page.getByText('Aceito',{exact:true}).count(),1,await page.locator('.submission-verdict-hero').innerText())
 await page.screenshot({path:output+`/accepted-${index+1}.png`})

 if(index===0) {
  await page.getByRole('link',{name:'Abrir resultado salvo →',exact:true}).click()
  await page.getByText('Aceito',{exact:true}).waitFor()
  assert.equal(await page.getByLabel('Respostas HTTP',{exact:true}).count(),0,'hidden evaluation responses remain protected')
  await page.getByRole('link',{name:'← voltar ao desafio',exact:true}).click()
  await page.locator('.monaco-editor textarea').waitFor()
  await page.waitForFunction(()=>document.querySelector('.monaco-editor .view-lines')?.textContent?.includes('biblioteca'))
 }
 console.log(`PASS activity ${index+1}: exploration, wrong answer, debugger, Monaco correction and acceptance`)
 }
 if(phase==='all' || phase==='progress') {
 await page.goto(listURL)
 await page.getByRole('heading',{name:pack.title,exact:true,level:1}).waitFor()
 assert.equal(await page.getByText('CONCLUÍDO',{exact:true}).count(),6)
 console.log('PASS correct answer: edit in Monaco, graded acceptance and list progress')
 }
 assert.deepEqual(errors,[])
} catch (error) { console.error(error); await page.screenshot({path:output+'/failure.png',fullPage:true}); throw error } finally { await browser.close() }
