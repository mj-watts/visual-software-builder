import { ref, computed, type Ref } from 'vue'
import { api } from './api'
import { emptyDraft, validateDraft, imageAllowed } from './domain'
import type { Draft, Ticket, Group } from './types'
export function useWorkspace(projectId:Ref<string>=ref('default')) {
  let refreshVersion=0
  const tickets=ref<Ticket[]>([]), selectedId=ref(''), creating=ref(false), draft=ref<Draft>(emptyDraft())
  const groups=ref<Group[]>([])
  const error=ref(''), busy=ref(false), loading=ref(true), online=ref(true)
  const selected=computed(()=>tickets.value.find(t=>t.id===selectedId.value))
  const editable=computed(()=>creating.value || selected.value?.status==='todo')
  const dirty=computed(()=>JSON.stringify(draft.value)!==JSON.stringify(toDraft(selected.value)))
  function select(ticket:Ticket) { selectedId.value=ticket.id;creating.value=false;draft.value=toDraft(ticket);error.value='' }
  function newTicket(groupId='') { selectedId.value='';creating.value=true;draft.value={...emptyDraft(),group_id:groupId};error.value='' }
  async function refresh() {
    const version=++refreshVersion,id=projectId.value
    if(!id){reset();loading.value=false;return}
    try {const [items,collections]=await Promise.all([api.list(id),api.groups(id)]);if(!currentRefresh(version,id))return;tickets.value=items;groups.value=collections;online.value=true}
    catch {if(currentRefresh(version,id))online.value=false}
    finally {if(currentRefresh(version,id))loading.value=false}
  }
  function currentRefresh(version:number,id:string) {return version===refreshVersion && id===projectId.value}
  function reset() {refreshVersion++;tickets.value=[];groups.value=[];selectedId.value='';creating.value=false;draft.value=emptyDraft();error.value=''}
  async function removeTicket(id:string) {
    await action(async()=>{await api.deleteTicket(id);if(selectedId.value===id){selectedId.value='';creating.value=false;draft.value=emptyDraft()};await refresh()})
  }
  async function removeTickets(ids:string[]) {
    return action(async()=>{
      await api.deleteTickets(ids,projectId.value)
      if(ids.includes(selectedId.value)){selectedId.value='';creating.value=false;draft.value=emptyDraft()}
      await refresh()
    })
  }
  async function action(fn:()=>Promise<void>) {
    busy.value=true;error.value=''
    try {await fn();return true} catch(e) {error.value=e instanceof Error ? e.message : 'Something went wrong.';return false}
    finally {busy.value=false}
  }
  async function persist() {
    const message=validateDraft(draft.value.title,draft.value.prompt)
    if(message) throw new Error(message)
    const saved=creating.value ? await api.create(draft.value,projectId.value) : await api.save(selectedId.value,draft.value)
    await refresh();select(saved)
  }
  async function save() { await action(persist) }
  async function run() {
    await action(async()=>{if(dirty.value) await persist();await api.run(selectedId.value);await refresh()})
  }
  async function cancel() { await action(async()=>{await api.cancel(selectedId.value);await refresh()}) }
  async function retry() { await action(async()=>{await api.retry(selectedId.value);await refresh()}) }
  async function drop(id:string) {
    const ticket=tickets.value.find(t=>t.id===id)
    if(ticket?.status!=='todo') return
    await action(async()=>{await api.run(id);await refresh();select(tickets.value.find(t=>t.id===id)! )})
  }
  async function runGroup(id:string) {
    await action(async()=>{
      if(selected.value?.group_id===id && editable.value && dirty.value)throw new Error('Save your ticket changes before running this group.')
      await api.runGroup(id);await refresh()
      const current=selected.value
      if(current?.group_id===id)select(current)
    })
  }
  async function saveGroup(id:string,value:Omit<Group,'id'>) {
    return action(async()=>{await api.saveGroup(id,value,projectId.value);await refresh()})
  }
  async function deleteGroup(id:string) {
    await action(async()=>{await api.deleteGroup(id);await refresh();if(draft.value.group_id===id)draft.value.group_id=''})
  }
  async function assignGroup(group_id:string) {
    await action(async()=>{await api.assignGroup(selectedId.value,group_id);draft.value.group_id=group_id;await refresh()})
  }
  async function moveToGroup(id:string,group_id:string) {
    if(!tickets.value.some(ticket=>ticket.id===id))return
    await action(async()=>{
      await api.assignGroup(id,group_id)
      if(selectedId.value===id)draft.value.group_id=group_id
      await refresh()
    })
  }
  async function attach(file:File) {
    if(!imageAllowed(file.type,file.size)) {error.value='Use PNG, JPEG or WebP images up to 5 MB.';return}
    if(draft.value.images.length>=6) {error.value='A ticket can have up to 6 images.';return}
    try {draft.value.images.push(await readImage(file))} catch {error.value='Could not read this image.'}
  }
  async function paste(event:ClipboardEvent) {
    if(!editable.value) return
    const files=Array.from(event.clipboardData?.files ?? [])
    if(!files.length) return
    event.preventDefault()
    for(const file of files) await attach(file)
  }
  return {selectedId,reset,removeTicket,removeTickets,groups,runGroup,saveGroup,deleteGroup,assignGroup,moveToGroup,tickets,selected,creating,draft,error,busy,loading,online,editable,dirty,select,newTicket,refresh,save,run,cancel,retry,drop,paste}
}
function toDraft(ticket?:Ticket):Draft {
  if(!ticket) return emptyDraft()
  return {title:ticket.title,prompt:ticket.prompt,provider:ticket.provider,images:[...ticket.images],workspace_id:ticket.workspace_id ?? '',permission:ticket.permission ?? 'read-only',allow_tests:ticket.allow_tests ?? false,group_id:ticket.group_id ?? ''}
}
function readImage(file:File):Promise<string> {
  return new Promise((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(String(reader.result));reader.onerror=reject;reader.readAsDataURL(file)})
}
