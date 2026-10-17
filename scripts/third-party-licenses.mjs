// Collect upstream license notices without publishing local cache paths.
import fs from 'node:fs';
import path from 'node:path';
import {spawnSync} from 'node:child_process';
const go=process.env.CERTPLUM_GO||'go';
const p=spawnSync(go,['list','-deps','-f','{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}','./cmd/certplum'],{encoding:'utf8',windowsHide:true});
if(p.status!==0)throw Error(p.stderr||'go list failed');
const modules=new Map();for(const line of p.stdout.split(/\r?\n/)){const [name,version,dir]=line.split('|');if(name&&version&&dir)modules.set(name,{version,dir});}
let out='Certplum - upstream license notices\n\nOriginal license texts; no local paths or user identities are included.\n\n';
for(const [name,{version,dir}] of [...modules].sort(([a],[b])=>a.localeCompare(b))){const files=fs.readdirSync(dir).filter(f=>/^(LICEN[CS]E|COPYING|NOTICE)([._-].*)?$/i.test(f)&&fs.statSync(path.join(dir,f)).isFile());if(!files.length)throw Error('No license found for '+name);out+='='.repeat(72)+'\n'+name+'@'+version+'\n';for(const file of files)out+='\n--- '+file+' ---\n'+fs.readFileSync(path.join(dir,file),'utf8').trim()+'\n';out+='\n';}
fs.writeFileSync('THIRD_PARTY_LICENSES.txt',out.split(/\r?\n/).map(line=>line.trimEnd()).join('\n').trimEnd()+'\n');console.log(`Collected notices for ${modules.size} runtime modules`);
