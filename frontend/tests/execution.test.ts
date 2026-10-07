import { describe, it, expect, vi, afterEach } from 'vitest'
import { useExecution } from '../src/execution'
afterEach(()=>vi.unstubAllGlobals())
const workspace={id:'ws-1',path:'/repo',name:'Example',test_preset:'none',test_directory:'.'}
describe('execution controls',()=>{
 it('refreshes a stalled live connection and stops polling when closed',async()=>{
  vi.useFakeTimers()
  vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,json:async()=>[]})))
  let source:any
  vi.stubGlobal('EventSource',class {onopen:any;onmessage:any;onerror:any;close=vi.fn();constructor(){source=this}})
  const changed=vi.fn(),e=useExecution(changed)
  try {
   e.start();source.onopen();changed.mockClear()
   await vi.advanceTimersByTimeAsync(2000)
   expect(e.live.value).toBe(true)
   expect(changed).toHaveBeenCalledTimes(1)
   e.stop();changed.mockClear()
   await vi.advanceTimersByTimeAsync(4000)
   expect(changed).not.toHaveBeenCalled()
  } finally {e.stop();vi.useRealTimers()}
 })
 it('loads provider readiness and registered workspaces',async()=>{vi.stubGlobal('fetch',vi.fn(async(url:string)=>({ok:true,json:async()=>url.endsWith('config')?{providers:[{id:'codex',available:true}],workspace_roots:['/repo'],timeout_seconds:60}:[workspace]})));const e=useExecution(vi.fn());await e.load();expect(e.providers.value[0].id).toBe('codex');expect(e.workspaces.value[0].name).toBe('Example')})
 it('saves repository settings and reports errors',async()=>{vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,json:async()=>workspace})));const e=useExecution(vi.fn());await e.register(workspace);expect(e.workspaces.value).toHaveLength(1);vi.stubGlobal('fetch',vi.fn().mockRejectedValue(new Error('invalid repository')));await e.register(workspace);expect(e.error.value).toBe('invalid repository')})
 it('loads run history and latest activity',async()=>{vi.stubGlobal('fetch',vi.fn(async(url:string)=>({ok:true,json:async()=>url.endsWith('/runs')?[{id:'run-1'}]:[{message:'Working'}]})));const e=useExecution(vi.fn());await e.history('SW-001');expect(e.runs.value[0].id).toBe('run-1');expect(e.events.value[0].message).toBe('Working');await e.history('');expect(e.runs.value).toHaveLength(0)})
 it('streams changes and cleans up connection',async()=>{vi.useFakeTimers();vi.stubGlobal('fetch',vi.fn(async(url:string)=>({ok:true,json:async()=>url.endsWith('/config')?{providers:[],workspace_roots:[],timeout_seconds:60}:[]})));let source:any;const changed=vi.fn();vi.stubGlobal('EventSource',class {onmessage:any;onopen:any;onerror:any;close=vi.fn();constructor(){source=this}});const e=useExecution(changed);e.start();source.onopen();expect(e.live.value).toBe(true);source.onmessage({data:'{"ticket_id":"SW-001"}'});await vi.advanceTimersByTimeAsync(200);expect(changed).toHaveBeenCalled();source.onerror();expect(e.live.value).toBe(false);source.onmessage({data:'bad json'});e.stop();expect(source.close).toHaveBeenCalled();vi.useRealTimers()})
})

import { mount, flushPromises } from '@vue/test-utils'
import WorkspaceSettings from '../src/components/WorkspaceSettings.vue'
import RunHistory from '../src/components/RunHistory.vue'
import TaskPane from '../src/components/TaskPane.vue'
import { useWorkspace } from '../src/workspace'
const snapshot={title:'Real task',prompt:'Build it',provider:'codex',images:[],workspace_id:'ws-1',permission:'workspace-write',allow_tests:false}
const run={id:'r1',ticket_id:'SW-001',snapshot,status:'failed',base_commit:'abc',worktree:'/runs/r1/worktree',created_at:'today',started_at:'today',finished_at:'today',response:'Failed',files:[{path:'file.txt',additions:1,deletions:1,diff:'+new\n-old'}],error:'Failed',tests:'not-requested'} as any
const providers=[{id:'codex',name:'Codex',available:true,reason:'Ready'},{id:'claude',name:'Claude',available:false,reason:'Set key'}] as any
it('registers a repository through settings',async()=>{
 const w=mount(WorkspaceSettings,{props:{open:true,workspaces:[workspace as any],providers,roots:['/repo'],timeout:60,error:'',busy:false}})
 await flushPromises();const form=document.querySelector('form')!;const inputs=form.querySelectorAll('input');inputs[0].value='Project';inputs[0].dispatchEvent(new Event('input',{bubbles:true}));inputs[1].value='/repo';inputs[1].dispatchEvent(new Event('input',{bubbles:true}));inputs[2].value='frontend';inputs[2].dispatchEvent(new Event('input',{bubbles:true}));const select=form.querySelector('select')!;select.value='vitest';select.dispatchEvent(new Event('change',{bubbles:true}));await flushPromises();form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await flushPromises();expect(w.emitted('register')?.[0][0]).toMatchObject({name:'Project',path:'/repo',test_preset:'vitest'});(document.querySelector('[aria-label="Close preview"]') as HTMLButtonElement).click();await flushPromises();w.unmount()
})
it('reviews run snapshots and historical file diffs',async()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,json:async()=>[{id:1,type:'agent',message:'Working'}]})))
 const w=mount(RunHistory,{props:{runs:[run],events:[{id:1,type:'status',message:'Failed'} as any],live:true}});await w.get('.history-row').trigger('click');await flushPromises();expect(document.body.textContent).toContain('/runs/r1/worktree');(document.querySelector('.run-review .file-row') as HTMLButtonElement).click();await flushPromises();expect(document.querySelector('.diff')?.textContent).toContain('+new');(document.querySelector('[aria-label="Close preview"]') as HTMLButtonElement).click();await flushPromises();w.unmount()
})
it('changes execution permissions and emits cancel and retry actions',async()=>{
 const draft={...snapshot} as any,ticket={...snapshot,id:'SW-001',status:'todo',files:[],response:''} as any
 const w=mount(TaskPane,{props:{ticket,draft,creating:false,editable:true,busy:false,dirty:false,error:'',providers,workspaces:[workspace as any],runs:[],events:[],live:false}})
 await w.get('#workspace').setValue('ws-1');await w.get('#permission').setValue('read-only');await w.get('[type="checkbox"]').setValue(true);await w.get('.text-button').trigger('click');expect(w.emitted('settings')).toBeTruthy();expect(draft.allow_tests).toBe(true)
 await w.setProps({ticket:{...ticket,status:'running'},editable:false});await w.get('.pane-actions button').trigger('click');expect(w.emitted('cancel')).toBeTruthy();await w.setProps({ticket:{...ticket,status:'cancelled'}});await w.get('.pane-actions button').trigger('click');expect(w.emitted('retry')).toBeTruthy();w.unmount()
})
it('invokes workspace retry and cancel APIs',async()=>{vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,json:async()=>[]})));const w=useWorkspace();w.select({id:'SW-001',...snapshot} as any);await w.cancel();await w.retry();expect(fetch).toHaveBeenCalledWith('/api/tickets/SW-001/retry',expect.objectContaining({method:'POST'}))})
