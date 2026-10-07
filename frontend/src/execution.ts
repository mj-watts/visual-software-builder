import { ref } from 'vue'
import { api } from './api'
import type { ProviderInfo, Workspace, Run, RunEvent } from './types'
const defaults:ProviderInfo[]=[{id:'demo',name:'Demo agent',available:true,reason:'Simulated'},{id:'codex',name:'Codex',available:false,reason:'Configure backend credentials'},{id:'claude',name:'Claude',available:false,reason:'Configure backend credentials'}]
export function useExecution(changed:()=>void) {
 const providers=ref<ProviderInfo[]>(defaults),workspaces=ref<Workspace[]>([]),roots=ref<string[]>([]),timeout=ref(600)
 const runs=ref<Run[]>([]),events=ref<RunEvent[]>([]),error=ref(''),busy=ref(false),live=ref(false)
 let stream:EventSource|undefined,timer:ReturnType<typeof setTimeout>|undefined,poll:ReturnType<typeof setInterval>|undefined,historyVersion=0,currentTicket=''
 async function load() {
  error.value=''
  try {const [config,list]=await Promise.all([api.config(),api.workspaces()]);providers.value=config.providers ?? defaults;roots.value=config.workspace_roots ?? [];timeout.value=config.timeout_seconds ?? 600;workspaces.value=Array.isArray(list)?list:[]}
  catch {error.value='Could not load execution settings. Check the backend connection.'}
 }
 async function register(value:Omit<Workspace,'id'>) {
  busy.value=true;error.value=''
  try {const saved=await api.register(value);workspaces.value=[...workspaces.value.filter(w=>w.id!==saved.id),saved]}
  catch(e) {error.value=e instanceof Error?e.message:'Could not register repository.'}
  finally {busy.value=false}
 }
 async function history(id:string) {
  const version=++historyVersion
  if(id!==currentTicket){runs.value=[];events.value=[];currentTicket=id}
  if(!id) {runs.value=[];events.value=[];return}
  try {const list=await api.history(id);if(version!==historyVersion)return;const activity=list[0]?await api.events(list[0].id):[];if(version!==historyVersion)return;runs.value=list;events.value=activity}
  catch {if(version===historyVersion){runs.value=[];events.value=[]}}
 }
 function message(event:MessageEvent) {
  try {JSON.parse(event.data)} catch {return}
  clearTimeout(timer);timer=setTimeout(changed,150)
 }
 function start() {
  stop()
  // An open stream can stall without firing onerror; keep the board reconciled.
  poll=setInterval(changed,2000)
  if(typeof EventSource==='undefined') return
  stream=new EventSource('/api/events')
  stream.onopen=()=>{live.value=true;load();changed()}
  stream.onerror=()=>{live.value=false}
  stream.onmessage=message
 }
 function stop() {stream?.close();clearTimeout(timer);clearInterval(poll);historyVersion++;live.value=false}
 return {providers,workspaces,roots,timeout,runs,events,error,busy,live,load,register,history,start,stop}
}
