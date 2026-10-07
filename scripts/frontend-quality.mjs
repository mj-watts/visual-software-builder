// ESLint computes cyclomatic complexity; V8 provides executable-line coverage.
import fs from 'node:fs'
import path from 'node:path'
import {createRequire} from 'node:module'
const require=createRequire(path.resolve('package.json'))
const {Linter}=require('eslint')
const parser=require('@typescript-eslint/parser')
const coverage=JSON.parse(fs.readFileSync('coverage/coverage-final.json','utf8'))
const linter=new Linter(),rows=[]
function functions(node, found=[]) {
 if(!node || typeof node!=='object') return found
 if(['FunctionDeclaration','FunctionExpression','ArrowFunctionExpression'].includes(node.type)) found.push(node)
 for(const [key,value] of Object.entries(node)) {
  if(['parent','tokens','comments'].includes(key)) continue
  if(Array.isArray(value)) value.forEach(child=>functions(child,found))
  else if(value && typeof value==='object') functions(value,found)
 }
 return found
}
for(const [file,data] of Object.entries(coverage)) {
 if(file.endsWith('types.ts')) continue
 let source=fs.readFileSync(file,'utf8')
 if(file.endsWith('.vue')) {
  const match=source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)
  source=match ? '\n'.repeat(source.slice(0,match.index).split('\n').length-1)+match[1] : ''
 }
 const ast=parser.parse(source,{loc:true,range:true,sourceType:'module',ecmaVersion:2022})
 const nodes=functions(ast)
 const messages=linter.verify(source,[{files:['**/*.{ts,vue}'],languageOptions:{parser,ecmaVersion:2022,sourceType:'module'},rules:{complexity:['error',0]}}],{filename:file})
 const parseErrors=messages.filter(m=>m.fatal)
 if(parseErrors.length) throw new Error(JSON.stringify(parseErrors))
 for(const m of messages.filter(m=>m.ruleId==='complexity')) {
  const node=nodes.find(n=>n.loc.start.line===m.line && n.loc.start.column===m.column-1) ?? nodes.find(n=>n.loc.start.line===m.line)
  if(!node) throw new Error(`Function not mapped: ${file}:${m.line}`)
  const complexity=Number(m.message.match(/complexity of (\d+)/)[1])
  const lines=new Map()
  for(const [id,stmt] of Object.entries(data.statementMap)) {
   if(stmt.start.line<node.loc.start.line || stmt.end.line>node.loc.end.line) continue
   for(let line=stmt.start.line;line<=stmt.end.line;line++) lines.set(line,(lines.get(line)??false)||data.s[id]>0)
  }
  const rate=lines.size ? [...lines.values()].filter(Boolean).length/lines.size : 0
  const crap=complexity**2*(1-rate)**3+complexity
  rows.push({file:path.relative(process.cwd(),file),function:node.id?.name ?? m.message.split(' has ')[0],line:m.line,complexity,coverage:Number(rate.toFixed(3)),crap:Number(crap.toFixed(3))})
 }
}
fs.writeFileSync('../docs/frontend-quality.json',JSON.stringify(rows,null,2)+'\n')
const failures=rows.filter(r=>r.crap>=10)
console.log(`${rows.length} TypeScript functions; maximum CRAP ${Math.max(...rows.map(r=>r.crap)).toFixed(3)}`)
if(failures.length){console.error(failures);process.exitCode=1}
