import type { Draft, Ticket, Workspace, Configuration, Run, RunEvent, Group, Project, ManagedProject } from './types'
function scope(path:string,id:string) {return id==='default'?path:`${path}?project_id=${encodeURIComponent(id)}`}
export async function request<T>(path:string,method='GET',body?:unknown):Promise<T> {
  const response = await fetch('/api'+path,{method,headers:{'Content-Type':'application/json'},body:body ? JSON.stringify(body) : undefined})
  if (!response.ok) throw new Error(await errorMessage(response))
  return response.json()
}
async function errorMessage(response:Response) {
  const body = await response.json().catch(()=>({}))
  if (typeof body.detail === 'string') return body.detail
  return `Request failed (${response.status}). Check your ticket and try again.`
}
export const api = {
  projects:()=>request<ManagedProject[]>('/projects?include_deleted=true'),
  createProject:(value:Project)=>request<ManagedProject>('/projects','POST',value),
  deleteProject:(id:string)=>request<ManagedProject>(`/projects/${id}`,'DELETE'),
  restoreProject:(id:string)=>request<ManagedProject>(`/projects/${id}/restore`,'POST'),
  updateProject:(id:string,value:Project)=>request<ManagedProject>(`/projects/${id}`,'PUT',value),
  project:(id='default')=>request<Project>(id==='default'?'/project':`/projects/${id}`),
  saveProject:(value:Project,id='default')=>request<Project>(id==='default'?'/project':`/projects/${id}`,'PUT',value),
  deleteTicket:(id:string)=>request<{deleted:string}>(`/tickets/${id}`,'DELETE'),
  deleteTickets:(ids:string[],project_id:string)=>request<{deleted:string[]}>('/tickets/delete','POST',{ids,project_id}),
  restoreTicket:(id:string)=>request<Ticket>(`/tickets/${id}/restore`,'POST'),
  deletedTickets:(id:string)=>request<Ticket[]>(`/tickets?project_id=${encodeURIComponent(id)}&deleted=true`),
  runGroup:(id:string)=>request<Ticket[]>(`/groups/${id}/run`,'POST'),
  groups:(projectId='default')=>request<Group[]>(scope('/groups',projectId)),
  saveGroup:(id:string,value:Omit<Group,'id'>,projectId='default')=>request<Group>(id?`/groups/${id}`:scope('/groups',projectId),id?'PATCH':'POST',value),
  deleteGroup:(id:string)=>request<{deleted:string}>(`/groups/${id}`,'DELETE'),
  assignGroup:(id:string,group_id:string)=>request<Ticket>(`/tickets/${id}/group`,'PUT',{group_id}),
  list:(id='default')=>request<Ticket[]>(scope('/tickets',id)),
  create:(draft:Draft,id='default')=>request<Ticket>(scope('/tickets',id),'POST',draft),
  save:(id:string,draft:Draft)=>request<Ticket>(`/tickets/${id}`,'PATCH',draft),
  config:()=>request<Configuration>('/config'),
  workspaces:()=>request<Workspace[]>('/workspaces'),
  register:(value:Omit<Workspace,'id'>)=>request<Workspace>('/workspaces','POST',value),
  history:(id:string)=>request<Run[]>(`/tickets/${id}/runs`),
  events:(id:string)=>request<RunEvent[]>(`/runs/${id}/events`),
  cancel:(id:string)=>request<Ticket>(`/tickets/${id}/cancel`,'POST'),
  retry:(id:string)=>request<Ticket>(`/tickets/${id}/retry`,'POST'),
  run:(id:string)=>request<Ticket>(`/tickets/${id}/run`,'POST'),
}
