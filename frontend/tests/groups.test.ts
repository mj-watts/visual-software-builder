import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import Board from '../src/components/Board.vue'
import { useWorkspace } from '../src/workspace'
import { api } from '../src/api'
const group={id:'g1',name:'Navbar',color:'orange' as const}
const ticket={id:'SW-001',title:'Links',prompt:'Add links',provider:'demo' as const,images:[],files:[],status:'todo' as const,response:'',created_at:'',queued_at:'',latest_run_id:'',workspace_id:'',permission:'read-only' as const,allow_tests:false,group_id:'g1'}
describe('ticket groups',()=>{
 it('forwards selection modifiers and visible grouped card order',async()=>{
  const w=mount(Board,{attachTo:document.body,props:{tickets:[ticket,{...ticket,id:'SW-002',group_id:''}],groups:[group],loading:false,selectedIds:[ticket.id]}})
  await w.get('[data-ticket="SW-002"]').trigger('click',{shiftKey:true})
  const [clicked,event,order]=w.emitted('select')![0] as any[]
  expect(clicked.id).toBe('SW-002');expect(event.shiftKey).toBe(true);expect(order).toEqual(['SW-001','SW-002'])
  expect(w.get('[data-ticket="SW-001"]').attributes('aria-pressed')).toBe('true');w.unmount()
 })
 it('handles Mac Ctrl-click context gestures as ticket selection',async()=>{
  const w=mount(Board,{props:{tickets:[ticket],groups:[group],loading:false}})
  await w.get('[data-ticket]').trigger('contextmenu',{ctrlKey:true})
  expect((w.emitted('select')?.[0]?.[1] as MouseEvent)?.ctrlKey).toBe(true);w.unmount()
 })
 it('drops into and out of groups without starting a run',async()=>{
  const w=mount(Board,{props:{tickets:[ticket],groups:[group],loading:false}})
  const dataTransfer={getData:()=>ticket.id}
  await w.get('[data-group="g1"]').trigger('drop',{dataTransfer})
  expect(w.emitted('assignGroup')?.[0]).toEqual([ticket.id,'g1'])
  expect(w.emitted('drop')).toBeUndefined()
  await w.get('[data-lane="todo"] .lane-header').trigger('drop',{dataTransfer})
  expect(w.emitted('assignGroup')?.[1]).toEqual([ticket.id,''])
  expect(w.find('.ungroup-drop').exists()).toBe(false)
  w.unmount()
 })
 it.each([['todo','todo'],['active','running'],['done','done']] as const)('ungroups on the %s column background without running',async(lane,status)=>{
  const w=mount(Board,{props:{tickets:[{...ticket,status}],groups:[group],loading:false}})
  await w.get(`[data-lane="${lane}"]`).trigger('drop',{dataTransfer:{getData:()=>ticket.id}})
  expect(w.emitted('assignGroup')?.[0]).toEqual([ticket.id,''])
  expect(w.emitted('drop')).toBeUndefined();w.unmount()
 })
 it('regroups a dragged ticket while preserving another selected draft',async()=>{
  vi.spyOn(api,'assignGroup').mockResolvedValue({...ticket,group_id:'g2'})
  vi.spyOn(api,'list').mockResolvedValue([{...ticket,group_id:'g2'},{...ticket,id:'SW-002'}])
  vi.spyOn(api,'groups').mockResolvedValue([group])
  const w=useWorkspace();w.tickets.value=[ticket,{...ticket,id:'SW-002'}];w.select(w.tickets.value[1]!);w.draft.value.prompt='Unsaved'
  try {
   await w.moveToGroup(ticket.id,'g2')
   expect(api.assignGroup).toHaveBeenCalledWith(ticket.id,'g2')
   expect(w.draft.value.prompt).toBe('Unsaved');expect(w.draft.value.group_id).toBe('g1')
   expect(w.selected.value?.id).toBe('SW-002')
   await w.moveToGroup('SW-002','g2');expect(w.draft.value.group_id).toBe('g2');expect(w.draft.value.prompt).toBe('Unsaved')
   vi.mocked(api.assignGroup).mockClear();await w.moveToGroup('external-drag','g2');expect(api.assignGroup).not.toHaveBeenCalled()
  } finally {vi.restoreAllMocks()}
 })
 it('allows completed tickets to move groups and stops active-lane drop bubbling',async()=>{
  const w=mount(Board,{props:{tickets:[{...ticket,status:'done'},{...ticket,id:'SW-002',status:'running'}],groups:[group],loading:false}})
  expect(w.get('[data-ticket="SW-001"]').attributes('draggable')).toBe('true')
  await w.get('[data-lane="active"] [data-group]').trigger('drop',{dataTransfer:{getData:()=>ticket.id}})
  expect(w.emitted('assignGroup')?.[0]).toEqual([ticket.id,'g1']);expect(w.emitted('drop')).toBeUndefined();w.unmount()
 })
 it('creates grouped drafts and manages durable metadata',async()=>{
 vi.spyOn(api,'list').mockResolvedValue([ticket]);vi.spyOn(api,'groups').mockResolvedValue([group])
 vi.spyOn(api,'saveGroup').mockResolvedValue(group);vi.spyOn(api,'deleteGroup').mockResolvedValue({deleted:'g1'});vi.spyOn(api,'assignGroup').mockResolvedValue({...ticket,group_id:''})
 const w=useWorkspace();await w.refresh();w.newTicket('g1');expect(w.draft.value.group_id).toBe('g1')
 expect(await w.saveGroup('',{name:'Navbar',color:'orange'})).toBe(true)
 w.select(ticket);w.draft.value.prompt='Unsaved prompt';await w.assignGroup('');expect(w.draft.value.prompt).toBe('Unsaved prompt');expect(w.draft.value.group_id).toBe('')
 await w.deleteGroup('g1');expect(api.deleteGroup).toHaveBeenCalledWith('g1');vi.restoreAllMocks()
 })
 it('shows empty Todo groups, collapses and offers group actions',async()=>{
 const w=mount(Board,{props:{tickets:[ticket],groups:[group],loading:false}});expect(w.findAll('[data-group="g1"]')).toHaveLength(1)
 await w.get('[aria-label="Minimise Navbar in Todo"]').trigger('click');expect(w.find('[data-ticket]').exists()).toBe(false)
 await w.get('[aria-label="Expand Navbar in Todo"]').trigger('click');await w.get('[aria-label="Add ticket to Navbar"]').trigger('click');expect(w.emitted('create')?.[0]).toEqual(['g1'])
 await w.get('[aria-label="Edit Navbar"]').trigger('click');expect(w.emitted('editGroup')?.[0]).toEqual([group])
 await w.get('[aria-label="Remove Navbar"]').trigger('click');expect(w.emitted('deleteGroup')?.[0]).toEqual(['g1'])
 await w.get('[aria-label="Add group"]').trigger('click');expect(w.emitted('createGroup')).toBeTruthy();w.unmount()
 })
 it('retains grouping in Done and leaves ungrouped tickets visible',()=>{
 const w=mount(Board,{props:{tickets:[{...ticket,status:'done'},{...ticket,id:'SW-002',group_id:''}],groups:[group],loading:false}})
 expect(w.get('[data-lane="done"] [data-group="g1"] [data-ticket]').attributes('data-ticket')).toBe(ticket.id)
 expect(w.get('[data-lane="todo"] > .lane-cards > [data-ticket]').attributes('data-ticket')).toBe('SW-002');w.unmount()
 })
})

import GroupDialog from '../src/components/GroupDialog.vue'
import { flushPromises } from '@vue/test-utils'
import TaskPane from '../src/components/TaskPane.vue'
it('validates group dialog and edits existing name and colour',async()=>{
 const w=mount(GroupDialog,{props:{open:false,busy:false,error:''}});await w.setProps({open:true});await flushPromises()
 const name=document.querySelector('#group-name') as HTMLInputElement;name.value='Hero';name.dispatchEvent(new Event('input',{bubbles:true}))
 ;(document.querySelector('[aria-label="Use blue colour"]') as HTMLButtonElement).click();await flushPromises()
 document.querySelector('.group-form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));expect(w.emitted('save')?.[0]).toEqual(['',{name:'Hero',color:'blue'}])
 await w.setProps({open:false});await w.setProps({group,open:true,error:'Save failed'});await flushPromises();expect(document.body.textContent).toContain('Save failed');expect((document.querySelector('#group-name') as HTMLInputElement).value).toBe('Navbar')
 ;(document.querySelector('[aria-label="Close preview"]') as HTMLButtonElement).click();await flushPromises();expect(w.emitted('update:open')).toBeTruthy();w.unmount()
})
it('assigns completed tickets without enabling prompt edits',async()=>{
 const w=mount(TaskPane,{props:{ticket:{...ticket,status:'done'},draft:{...ticket},groups:[group],creating:false,editable:false,busy:false,dirty:false,error:'',providers:[],workspaces:[],runs:[],events:[],live:false}})
 await w.get('#ticket-group').setValue('');expect(w.emitted('assignGroup')?.[0]).toEqual(['']);expect(w.get('textarea').attributes()).toHaveProperty('readonly');w.unmount()
})
it('uses group API routes and reports failed changes',async()=>{
 vi.stubGlobal('fetch',vi.fn().mockResolvedValue({ok:true,json:async()=>group}))
 await api.saveGroup('',{name:'Navbar',color:'orange'});expect(fetch).toHaveBeenLastCalledWith('/api/groups',expect.objectContaining({method:'POST'}))
 await api.saveGroup('g1',{name:'Nav',color:'blue'});expect(fetch).toHaveBeenLastCalledWith('/api/groups/g1',expect.objectContaining({method:'PATCH'}))
 await api.deleteGroup('g1');expect(fetch).toHaveBeenLastCalledWith('/api/groups/g1',expect.objectContaining({method:'DELETE'}))
 await api.assignGroup(ticket.id,'g1');expect(fetch).toHaveBeenLastCalledWith('/api/tickets/SW-001/group',expect.objectContaining({method:'PUT',body:'{"group_id":"g1"}'}))
 vi.stubGlobal('fetch',vi.fn().mockRejectedValue(new Error('offline')));const w=useWorkspace();expect(await w.saveGroup('',{name:'Nav',color:'blue'})).toBe(false);expect(w.error.value).toBe('offline');vi.unstubAllGlobals()
})

it('chooses custom colours and reopens them for editing',async()=>{
 const w=mount(GroupDialog,{props:{open:false,busy:false,error:''}});await w.setProps({open:true});await flushPromises()
 const picker=document.querySelector('[aria-label="Custom group colour"]') as HTMLInputElement
 expect(picker.type).toBe('color');picker.value='#efc123';picker.dispatchEvent(new Event('input',{bubbles:true}));await flushPromises()
 document.querySelector('.group-form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));expect(w.emitted('save')?.[0]).toEqual(['',{name:'',color:'#efc123'}])
 await w.setProps({open:false});await w.setProps({group:{...group,color:'#efc123'},open:true});await flushPromises();expect((document.querySelector('[aria-label="Custom group colour"]') as HTMLInputElement).value).toBe('#efc123');w.unmount()
})

import { groupAppearance, groupHex } from '../src/groupColors'
it('renders readable custom headers with circular icon controls',()=>{
 expect(groupHex('orange')).toBe('#ff7043');expect(groupHex('#ffffff')).toBe('#ffffff')
 expect(groupAppearance('#ffffff')['--group-ink']).toBe('#000000')
 expect(groupAppearance('#000000')['--group-ink']).toBe('#ffffff')
 const w=mount(Board,{props:{tickets:[ticket],groups:[{...group,color:'#ffffff'}],loading:false}})
 expect(w.get('[data-group]').attributes('style')).toContain('--group-color: #ffffff')
 const minimise=w.get('[aria-label="Minimise Navbar in Todo"]');expect(minimise.classes()).toContain('group-control');expect(minimise.find('svg').exists()).toBe(true);expect(minimise.text()).toBe('')
 expect(w.get('summary.group-control').find('svg').exists()).toBe(true);w.unmount()
})
it('starts a group through its menu and refreshes locked prompts',async()=>{
 const board=mount(Board,{props:{tickets:[ticket],groups:[group],loading:false}})
 await board.get('[aria-label="Run Navbar group"]').trigger('click');expect(board.emitted('runGroup')?.[0]).toEqual(['g1'])
 await board.setProps({tickets:[]});expect(board.get('[aria-label="Run Navbar group"]').attributes()).toHaveProperty('disabled');board.unmount()
 vi.spyOn(api,'runGroup').mockResolvedValue([{...ticket,status:'queued'}]);vi.spyOn(api,'list').mockResolvedValue([{...ticket,status:'queued'}]);vi.spyOn(api,'groups').mockResolvedValue([group])
 const w=useWorkspace();w.tickets.value=[ticket];w.select(ticket);w.draft.value.prompt='Unsaved';await w.runGroup('g1');expect(w.error.value).toContain('Save your ticket');expect(api.runGroup).not.toHaveBeenCalled();w.draft.value.prompt=ticket.prompt;await w.runGroup('g1');expect(w.selected.value?.status).toBe('queued');expect(w.editable.value).toBe(false);expect(api.runGroup).toHaveBeenCalledWith('g1');vi.restoreAllMocks()
 vi.stubGlobal('fetch',vi.fn().mockResolvedValue({ok:true,json:async()=>[]}));await api.runGroup('g1');expect(fetch).toHaveBeenCalledWith('/api/groups/g1/run',expect.objectContaining({method:'POST'}));vi.unstubAllGlobals()
})
