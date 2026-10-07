import { ref, computed } from 'vue'
import { api } from './api'
import type { ManagedProject, Project, Ticket } from './types'
export function useProjects() {
 const id=ref(readSelection())
 const items=ref<ManagedProject[]>([]),trash=ref<Ticket[]>([]),error=ref(''),busy=ref(false)
 const active=computed(()=>items.value.find(p=>p.id===id.value && !p.deleted))
 function select(value:string) {id.value=value;saveSelection(value)}
 async function action(fn:()=>Promise<void>) {
  busy.value=true;error.value=''
  try {await fn();return true} catch(e){error.value=e instanceof Error?e.message:'Could not update projects.';return false}
  finally {busy.value=false}
 }
 async function load() {
  return action(async()=>{items.value=await api.projects();if(!active.value)select(items.value.find(p=>!p.deleted)?.id ?? '')})
 }
 function replace(value:ManagedProject) {items.value=[...items.value.filter(p=>p.id!==value.id),value]}
 async function create(value:Project) {return action(async()=>{const saved=await api.createProject(value);replace(saved);select(saved.id)})}
 async function update(key:string,value:Project) {return action(async()=>{replace(await api.updateProject(key,value))})}
 async function remove(key:string) {
  return action(async()=>{replace(await api.deleteProject(key));if(id.value===key)select(items.value.find(p=>!p.deleted)?.id ?? '')})
 }
 async function restore(key:string) {return action(async()=>{replace(await api.restoreProject(key));select(key)})}
 async function loadTrash() {
  const selected=id.value;trash.value=[]
  return action(async()=>{const items=selected?await api.deletedTickets(selected):[];if(selected===id.value)trash.value=items})
 }
 async function restoreTicket(key:string) {return action(async()=>{await api.restoreTicket(key);trash.value=trash.value.filter(t=>t.id!==key)})}
 return {id,items,active,trash,error,busy,select,load,create,update,remove,restore,loadTrash,restoreTicket}
}

function readSelection() {try{return window.localStorage?.getItem('swimlane-project') ?? 'default'}catch{return 'default'}}
function saveSelection(value:string) {try{window.localStorage?.setItem('swimlane-project',value)}catch{/* Project switching still works when browser storage is unavailable. */}}
