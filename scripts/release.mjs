// Local-only distribution build. Does not install or publish anything.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {spawnSync} from 'node:child_process';
const version=process.argv[2]||'0.1.0';
if(!/^\d+\.\d+\.\d+(?:-[a-z0-9.]+)?$/.test(version))throw Error('Invalid version');
const go=process.env.CERTPLUM_GO||'go',nfpm=process.env.CERTPLUM_NFPM||'nfpm';
fs.mkdirSync('dist',{recursive:true});
function run(tool,args,extra={}){const p=spawnSync(tool,args,{stdio:'inherit',windowsHide:true,env:{...process.env,...extra}});if(p.error)throw p.error;if(p.status!==0)throw Error(`${tool} failed (${p.status})`);}
const outputs=[];
run(process.execPath,['scripts/third-party-licenses.mjs']);
for(const arch of ['amd64','arm64']){
 const bin=`build/linux_${arch}/certplum`;fs.mkdirSync(path.dirname(bin),{recursive:true});
 run(go,['build','-trimpath','-buildvcs=false','-ldflags',`-X main.version=${version}`,'-o',bin,'./cmd/certplum'],{GOOS:'linux',GOARCH:arch,CGO_ENABLED:'0'});
 const deb=`dist/certplum_${version}_${arch}.deb`;
 const manifest=fs.readFileSync('packaging/nfpm.yaml','utf8').replaceAll('${GOARCH}',arch).replaceAll('${CERTPLUM_VERSION}',version),config=`build/nfpm-${arch}.yaml`;fs.writeFileSync(config,manifest);
 run(nfpm,['package','--config',config,'--packager','deb','--target',deb],{GOARCH:arch,CERTPLUM_VERSION:version});outputs.push(deb);
}
const exe=`dist/certplum_${version}_windows_amd64.exe`;
run(go,['build','-trimpath','-buildvcs=false','-ldflags',`-X main.version=${version}`,'-o',exe,'./cmd/certplum'],{GOOS:'windows',GOARCH:'amd64',CGO_ENABLED:'0'});outputs.push(exe);
const sums=outputs.map(file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')+'  '+path.basename(file)).join('\n')+'\n';fs.writeFileSync(`dist/certplum_${version}_SHA256SUMS.txt`,sums);
console.log('Build complete. Files in dist/. Windows EXE is portable, not a setup wizard.');
