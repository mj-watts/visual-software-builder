import { it,expect,vi,afterEach } from 'vitest'
import { mount,flushPromises } from '@vue/test-utils'
import { useTicketSelection } from '../src/ticketSelection'
import { useWorkspace } from '../src/workspace'
import { api } from '../src/api'
import BulkTicketActions from '../src/components/BulkTicketActions.vue'
const plain={shiftKey:false,ctrlKey:false,metaKey:false}
const ticket={id:'SW-001',title:'Task',prompt:'Prompt',status:'todo',images:[],files:[],provider:'demo'} as any
const wrappers:any[]=[]
afterEach(()=>{wrappers.forEach(w=>w.unmount());wrappers.length=0;vi.restoreAllMocks()})
it('selects inclusive ranges in displayed order and reverses around the anchor',()=>{
 const s=useTicketSelection(),order=['c','a','b','d']
 expect(s.click('a',plain,order)).toBe(true)
 expect(s.click('d',{...plain,shiftKey:true},order)).toBe(false);expect(s.ids.value).toEqual(['a','b','d'])
 s.click('c',{...plain,shiftKey:true},order);expect(s.ids.value).toEqual(['c','a'])
 s.click('b',plain,order);expect(s.ids.value).toEqual([])
})
it('toggles arbitrary tickets with Ctrl or Cmd, adds ranges and handles missing anchors',()=>{
 const s=useTicketSelection(),order=['a','b','c','d']
 s.click('a',{...plain,ctrlKey:true},order);s.click('d',{...plain,metaKey:true},order);expect(s.ids.value).toEqual(['a','d'])
 s.click('b',{...plain,ctrlKey:true,shiftKey:true},order);expect(s.ids.value).toEqual(['a','d','b','c'])
 s.click('a',{...plain,ctrlKey:true},order);expect(s.ids.value).toEqual(['d','b','c'])
 s.retain(['b']);expect(s.ids.value).toEqual(['b']);s.clear()
 s.click('c',{...plain,shiftKey:true},order);expect(s.ids.value).toEqual(['c'])
})
it('deletes selected tickets through one scoped request and clears the inspector',async()=>{
 vi.spyOn(api,'deleteTickets').mockResolvedValue({deleted:[ticket.id]});vi.spyOn(api,'list').mockResolvedValue([]);vi.spyOn(api,'groups').mockResolvedValue([])
 const w=useWorkspace();w.tickets.value=[ticket];w.select(ticket)
 expect(await w.removeTickets([ticket.id])).toBe(true)
 expect(api.deleteTickets).toHaveBeenCalledWith([ticket.id],'default');expect(w.selectedId.value).toBe('');expect(w.tickets.value).toEqual([])
})
it('retains selection and edits when bulk deletion is rejected',async()=>{
 vi.spyOn(api,'deleteTickets').mockRejectedValue(new Error('Cancel running tickets first'))
 const w=useWorkspace();w.tickets.value=[ticket];w.select(ticket);w.draft.value.prompt='Unsaved'
 expect(await w.removeTickets([ticket.id])).toBe(false);expect(w.selectedId.value).toBe(ticket.id);expect(w.draft.value.prompt).toBe('Unsaved');expect(w.error.value).toContain('Cancel running')
})
async function click(name:string) {const button=Array.from(document.querySelectorAll('button')).find(b=>b.textContent?.trim()===name)!;button.click();await flushPromises()}
it('requires confirmation, supports cancellation and explains unsaved edits and restore',async()=>{
 const w=mount(BulkTicketActions,{attachTo:document.body,props:{tickets:[ticket],busy:false,error:'',unsaved:true}});wrappers.push(w)
 await click('Delete all');expect(w.emitted('remove')).toBeUndefined();expect(document.body.textContent).toContain('You can restore');expect(document.body.textContent).toContain('Unsaved edits')
 await click('Cancel');expect(w.emitted('remove')).toBeUndefined()
 await click('Delete all');await click('Confirm delete all');expect(w.emitted('remove')?.[0]).toEqual([[ticket.id]])
 await w.setProps({error:'Rejected'});expect(document.body.textContent).toContain('Rejected')
})
it('disables deletion for active runs, oversized selection and while busy',async()=>{
 const w=mount(BulkTicketActions,{props:{tickets:[{...ticket,status:'running'}],busy:false,error:'',unsaved:false}});wrappers.push(w)
 expect(w.get('.danger-button').attributes()).toHaveProperty('disabled')
 await w.setProps({tickets:Array.from({length:201},()=>ticket)});expect(w.text()).toContain('Select up to 200')
 await w.setProps({tickets:[ticket],busy:true});expect(w.get('.danger-button').attributes()).toHaveProperty('disabled')
 await w.setProps({busy:false});await w.get('[aria-label="Clear ticket selection"]').trigger('click');expect(w.emitted('clear')).toBeTruthy()
})
