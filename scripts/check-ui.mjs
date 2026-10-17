// Fictitious data only. Run against TestBrowserHarness, never against real data.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { spawn } from 'node:child_process';
const require=createRequire(path.resolve('.tools/ui/package.json'));
const { chromium }=require('playwright');
const dir=path.resolve('.local/ui');
fs.mkdirSync(dir,{recursive:true});
for(const file of ['demo.json','done'])fs.rmSync(path.join(dir,file),{force:true});
const harness=spawn(process.env.CLINICAL_GO||'go',['test','./internal/httpserver','-run','TestBrowserHarness','-v','-count=1'],{env:{...process.env,CLINICAL_UI_TEST_DIR:dir},windowsHide:true});
let harnessOutput='';harness.stdout.on('data',b=>harnessOutput+=b);harness.stderr.on('data',b=>harnessOutput+=b);
const harnessDone=new Promise(resolve=>harness.on('exit',code=>resolve(code)));
for(let i=0;!fs.existsSync(path.join(dir,'demo.json'));i++){if(i>600||harness.exitCode!==null)throw Error('No inició el arnés: '+harnessOutput);await new Promise(r=>setTimeout(r,100));}
const {url,totp}=JSON.parse(fs.readFileSync(path.join(dir,'demo.json'),'utf8'));
if(new URL(url).hostname!=='127.0.0.1'||new URL(url).protocol!=='https:')throw Error('Solo se permite el arnés HTTPS local de pruebas');
const browser=await chromium.launch({executablePath:process.env.CLINICAL_CHROME||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:1100},acceptDownloads:true});
const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
try{
 await page.goto(url);await page.waitForFunction(()=>typeof csrf!=='undefined'&&csrf.length===64);
 await page.getByLabel('Contraseña',{exact:true}).fill('contraseña local de prueba');
 await page.getByLabel('Frase de desbloqueo',{exact:true}).fill('frase larga y diferente de prueba local');
 await page.getByLabel('Código de tu autenticador').fill(totp);
 await page.getByRole('button',{name:'Desbloquear consulta'}).click();
 await page.getByText('Consulta desbloqueada.',{exact:true}).waitFor();
 await page.screenshot({path:path.join(dir,'editor.png'),fullPage:true});
 await page.getByLabel('Paciente',{exact:true}).fill('Persona Ficticia Álvarez');
 await page.getByLabel('RUT',{exact:true}).fill('12.345.678-5');
 await page.getByLabel('Contenido',{exact:true}).fill('Certificado de prueba visual. Atención clínica, evolución y coordinación. Sin datos reales.');
 await page.getByRole('button',{name:'Emitir y firmar PDF'}).click();
 await page.getByText('Documento emitido y firmado. Puedes descargarlo desde el historial.',{exact:true}).waitFor();
 const downloadPromise=page.waitForEvent('download');await page.getByRole('button',{name:'Descargar PDF'}).click();
 const download=await downloadPromise;const pdf=path.join(dir,'descarga-prueba.pdf');await download.saveAs(pdf);
 const href=await page.getByRole('link',{name:'Verificación pública'}).getAttribute('href');
 const verify=await context.newPage();await verify.goto(url+href);await verify.getByText('Documento registrado · VIGENTE',{exact:true}).waitFor();
 await verify.getByLabel('Archivo PDF').setInputFiles(pdf);await verify.getByText('El archivo coincide exactamente con el PDF emitido.',{exact:true}).waitFor();
 await verify.screenshot({path:path.join(dir,'verificacion.png'),fullPage:true});
 await page.screenshot({path:path.join(dir,'historial.png'),fullPage:true});
 page.once('dialog',dialog=>dialog.accept());await page.getByRole('button',{name:'Revocar',exact:true}).click();await page.getByText('Documento revocado.',{exact:true}).waitFor();
 await verify.reload();await verify.getByText('Documento registrado · REVOCADO',{exact:true}).waitFor();
 await page.getByRole('button',{name:'Bloquear',exact:true}).click();await page.getByText('Consulta bloqueada.',{exact:true}).waitFor();
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:path.join(dir,'movil-bloqueado.png'),fullPage:true});
 if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth))throw Error('Mobile overflow');
 if(errors.length)throw Error(errors.join('\n'));
 console.log('HTTPS: login, emisión, firma, descarga, hash local, revocación, bloqueo y ancho móvil: OK');
}catch(e){await page.screenshot({path:path.join(dir,'error.png'),fullPage:true});console.error('Estado visible:',await page.locator('#message').textContent());throw e;}
finally{await browser.close();fs.writeFileSync(path.join(dir,'done'),'done');const code=await harnessDone;if(code!==0)console.error(harnessOutput);}
