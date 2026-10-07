import type { Ticket } from './types'
export function validateDraft(title:string,prompt:string) {
  if (!title.trim()) return 'Give your ticket a title.'
  if (!prompt.trim()) return 'Write a prompt before running this ticket.'
  return ''
}
export function visibleTickets(tickets:Ticket[],query:string,filter:string) {
  return tickets.filter(ticket => matchesQuery(ticket,query) && matchesStatus(ticket.status,filter))
}
function matchesQuery(ticket:Ticket,query:string) {
  return `${ticket.id} ${ticket.title} ${ticket.prompt}`.toLowerCase().includes(query.toLowerCase())
}
export function matchesStatus(status:string,filter:string) {
  if (filter === 'all') return true
  if (filter === 'active') return ['queued','running','failed','cancelled'].includes(status)
  return status === filter
}
export function diffClass(line:string) {
  if (line.startsWith('+')) return 'added'
  if (line.startsWith('-')) return 'removed'
  if (line.startsWith('@@')) return 'hunk'
  return 'context'
}
export function imageAllowed(type:string,size:number) {
  return ['image/png','image/jpeg','image/webp'].includes(type) && size <= 5_000_000
}
export function emptyDraft() { return {title:'',prompt:'',provider:'demo' as const,images:[] as string[],workspace_id:'',permission:'read-only' as const,allow_tests:false,group_id:''} }
