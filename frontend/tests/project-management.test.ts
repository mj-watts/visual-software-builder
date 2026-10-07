import { it,expect,vi,afterEach,beforeEach } from 'vitest'
import { ref } from 'vue'
import { useWorkspace } from '../src/workspace'
import { useProjects } from '../src/projects'
import { api } from '../src/api'
beforeEach(()=>{const data=new Map<string,string>();vi.stubGlobal('localStorage',{getItem:(key:string)=>data.get(key) ?? null,setItem:(key:string,value:string)=>data.set(key,value)})})
const mounted:Array<{unmount:()=>void}>=[]
afterEach(()=>{mounted.splice(0).forEach(w=>w.unmount());vi.restoreAllMocks();vi.unstubAllGlobals()})
it('creates and switches projects and recovers deleted projects',async()=>{
 const original={id:'default',name:'Original',description:'',deleted:false},second={id:'second',name:'Second',description:'Details',deleted:false}
 vi.spyOn(api,'projects').mockResolvedValue([original]);vi.spyOn(api,'createProject').mockResolvedValue(second)
 vi.spyOn(api,'deleteProject').mockResolvedValue({...second,deleted:true});vi.spyOn(api,'restoreProject').mockResolvedValue(second)
 const p=useProjects();await p.load();expect(p.active.value).toEqual(original)
 vi.mocked(api.projects).mockResolvedValue([original,second]);await p.create({name:'Second',description:'Details'});expect(p.id.value).toBe('second')
 vi.mocked(api.projects).mockResolvedValue([original,{...second,deleted:true}]);await p.remove('second');expect(p.id.value).toBe('default')
 vi.mocked(api.projects).mockResolvedValue([original,second]);await p.restore('second');expect(p.id.value).toBe('second')
 expect(window.localStorage.getItem('swimlane-project')).toBe('second')
})
it('deletes only the selected ticket and refreshes project-scoped data',async()=>{
 const id=ref('second'),w=useWorkspace(id)
 const ticket={id:'SW-002',title:'Task',prompt:'Prompt',provider:'demo',images:[],files:[],status:'todo',group_id:'g1'} as any
 vi.spyOn(api,'list').mockResolvedValue([]);vi.spyOn(api,'groups').mockResolvedValue([]);vi.spyOn(api,'deleteTicket').mockResolvedValue({deleted:ticket.id})
 w.tickets.value=[ticket];w.select(ticket);await w.removeTicket(ticket.id)
 expect(api.deleteTicket).toHaveBeenCalledWith(ticket.id);expect(api.list).toHaveBeenCalledWith('second')
 expect(w.selectedId.value).toBe('');expect(w.creating.value).toBe(false)
})

import { mount,flushPromises } from '@vue/test-utils'
import ProjectManager from '../src/components/ProjectManager.vue'
import App from '../src/App.vue'
it('opens a checked project dropdown and protects unsaved edits before switching',async()=>{
 const projects=[{id:'default',name:'Original',description:'',deleted:false},{id:'other',name:'Other',description:'',deleted:false},{id:'gone',name:'Gone',description:'',deleted:true}]
 const ticket={id:'SW-001',title:'Task',prompt:'Prompt',provider:'demo',images:[],files:[],status:'todo',response:''} as any
 vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,json:async()=>[]})))
 vi.spyOn(api,'projects').mockResolvedValue(projects);vi.spyOn(api,'project').mockImplementation(async id=>projects.find(p=>p.id===id)!)
 vi.spyOn(api,'list').mockImplementation(async id=>id==='default'?[ticket]:[]);vi.spyOn(api,'groups').mockResolvedValue([])
 const w=mount(App,{attachTo:document.body});mounted.push(w);await flushPromises()
 await w.get('[aria-label="Switch project"]').trigger('click',{button:0,ctrlKey:false});await flushPromises()
 const current=document.querySelector('[data-project-option="default"]')!
 expect(current.getAttribute('aria-checked')).toBe('true')
 expect(document.querySelector('[data-project-option="gone"]')).toBeNull()
 expect(w.find('.projects-page').exists()).toBe(false)
 ;(document.querySelector('[data-project-option="other"]') as HTMLElement).click();await flushPromises()
 expect(w.get('h1').text()).toBe('Other')
 await w.get('[aria-label="Switch project"]').trigger('click',{button:0,ctrlKey:false});await flushPromises()
 ;(document.querySelector('[data-project-option="default"]') as HTMLElement).click();await flushPromises()
 await w.get('[data-ticket]').trigger('click');await w.get('#prompt').setValue('Unsaved')
 await w.get('[aria-label="Switch project"]').trigger('click',{button:0,ctrlKey:false});await flushPromises()
 ;(document.querySelector('[data-project-option="other"]') as HTMLElement).click();await flushPromises()
 await click('Keep editing');expect(w.get('#prompt').element.value).toBe('Unsaved');expect(w.get('h1').text()).toBe('Original')
 await w.get('[aria-label="Switch project"]').trigger('click',{button:0,ctrlKey:false});await flushPromises()
 ;(document.querySelector('[data-project-option="other"]') as HTMLElement).click();await flushPromises();await click('Discard changes')
 expect(w.get('h1').text()).toBe('Other');expect(w.find('.projects-page').exists()).toBe(false)
})
import ProjectDialog from '../src/components/ProjectDialog.vue'
it('shows a project page with screenshot previews instead of an overlay',async()=>{
 const image='data:image/png;base64,aW1hZ2U='
 const w=mount(ProjectManager,{attachTo:document.body,props:{page:true,open:true,projects:[{id:'default',name:'Preview',description:'App preview',deleted:false,screenshot:image}],selectedId:'default',trash:[],busy:false,error:''}});mounted.push(w)
 await flushPromises();expect(w.find('main.projects-page').exists()).toBe(true)
 expect(w.get('img').attributes('src')).toBe(image)
 expect(document.querySelector('.dialog-overlay')).toBeNull()
})
it('pastes, previews and removes a screenshot in the project editor',async()=>{
 const w=mount(ProjectDialog,{props:{open:true,project:{name:'Preview',description:''},busy:false,error:''}});mounted.push(w);await flushPromises()
 const area=document.querySelector('[aria-label="Project screenshot"]')!
 const event=new Event('paste',{bubbles:true,cancelable:true})
 Object.defineProperty(event,'clipboardData',{value:{files:[new File(['image'],'preview.png',{type:'image/png'})]}})
 area.dispatchEvent(event);await new Promise(resolve=>setTimeout(resolve,30));await flushPromises()
 expect(document.querySelector('.project-screenshot img')?.getAttribute('src')).toBe('data:image/png;base64,aW1hZ2U=')
 document.querySelector('.project-form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await flushPromises()
 expect(w.emitted('save')?.[0]).toEqual([{name:'Preview',description:'',screenshot:'data:image/png;base64,aW1hZ2U='}])
 await click('Remove screenshot');document.querySelector('.project-form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await flushPromises()
 expect(w.emitted('save')?.[1]).toEqual([{name:'Preview',description:'',screenshot:''}])
})
it('rejects unsupported screenshot uploads without replacing the current image',async()=>{
 const w=mount(ProjectDialog,{props:{open:true,project:{name:'Preview',description:'',screenshot:'data:image/png;base64,aW1hZ2U='},busy:false,error:''}});mounted.push(w);await flushPromises()
 const input=document.querySelector('#project-screenshot-file') as HTMLInputElement
 Object.defineProperty(input,'files',{value:[new File(['text'],'preview.svg',{type:'image/svg+xml'})]})
 input.dispatchEvent(new Event('change',{bubbles:true}));await flushPromises()
 expect(document.body.textContent).toContain('Use PNG, JPEG or WebP up to 5 MB.')
 expect(document.querySelector('.project-screenshot img')?.getAttribute('src')).toBe('data:image/png;base64,aW1hZ2U=')
})
async function click(text:string) {const button=Array.from(document.querySelectorAll('button')).find(b=>b.textContent?.trim()===text || b.getAttribute('aria-label')===text)!;button.click();await flushPromises()}
async function title(text:string) {const input=document.querySelector('#project-name') as HTMLInputElement;input.value=text;input.dispatchEvent(new Event('input',{bubbles:true}));await flushPromises()}
it('manages project forms and exposes restore actions in the dialog',async()=>{
 const original={id:'default',name:'Original',description:'Details',deleted:false},other={id:'other',name:'Other',description:'',deleted:false},deleted={id:'deleted',name:'Deleted',description:'',deleted:true}
 const w=mount(ProjectManager,{props:{open:true,projects:[original,other,deleted],selectedId:'default',trash:[{id:'SW-001',title:'Task'} as any],busy:false,error:''}});mounted.push(w)
 await flushPromises();await click('Edit Original');await title('Renamed')
 document.querySelector('.project-form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await flushPromises()
 expect(w.emitted('save')?.[0]).toEqual(['default',{name:'Renamed',description:'Details'}]);w.vm.saved();await flushPromises()
 await click('New project');await title('New')
 document.querySelector('.project-form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await flushPromises()
 expect(w.emitted('save')?.[1]).toEqual(['',{name:'New',description:''}]);w.vm.saved();await flushPromises()
 await click('Open Other');await click('Delete Other');await click('Restore Deleted');await click('Restore Task')
 expect(w.emitted('select')?.[0]).toEqual(['other']);expect(w.emitted('remove')?.[0]).toEqual(['other'])
 expect(w.emitted('restore')?.[0]).toEqual(['deleted']);expect(w.emitted('restoreTicket')?.[0]).toEqual(['SW-001'])
 await w.setProps({projects:[],trash:[],error:'Offline'});expect(document.body.textContent).toContain('No projects yet')
 await click('Close preview');expect(w.emitted('update:open')).toBeTruthy()
})
it('guards unsaved ticket edits before management and wires project creation and editing',async()=>{
 const original={id:'default',name:'Original',description:'',deleted:false},second={id:'second',name:'Second',description:'',deleted:false}
 const ticket={id:'SW-001',title:'Task',prompt:'Prompt',provider:'demo',images:[],files:[],status:'todo',response:''} as any
 vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,json:async()=>[]})))
 vi.spyOn(api,'projects').mockResolvedValue([original]);vi.spyOn(api,'project').mockImplementation(async id=>id==='second'?second:original)
 vi.spyOn(api,'list').mockResolvedValue([ticket]);vi.spyOn(api,'groups').mockResolvedValue([]);vi.spyOn(api,'deletedTickets').mockResolvedValue([])
 vi.spyOn(api,'createProject').mockResolvedValue(second);vi.spyOn(api,'updateProject').mockResolvedValue({...second,name:'Renamed'})
 const w=mount(App,{attachTo:document.body});mounted.push(w);await flushPromises();await w.get('[data-ticket]').trigger('click');await w.get('#prompt').setValue('Unsaved')
 await w.get('.brand').trigger('click');await flushPromises();expect(document.body.textContent).toContain('Discard unsaved ticket changes?')
 await click('Keep editing');expect(w.get('#prompt').element.value).toBe('Unsaved')
 await w.get('.workspace-link').trigger('click');await flushPromises();await click('Discard changes')
 await click('New project');await title('Second');document.querySelector('.project-form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await flushPromises()
 expect(api.createProject).toHaveBeenCalledWith({name:'Second',description:''})
 await click('Edit Second');await title('Renamed');document.querySelector('.project-form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await flushPromises()
 expect(api.updateProject).toHaveBeenCalledWith('second',{name:'Renamed',description:''})
 await click('Back to board');await w.get('.brand').trigger('click');await flushPromises();expect(w.find('main.projects-page').exists()).toBe(true)
})
it('restores deleted tickets, handles empty projects and surfaces API errors',async()=>{
 const p=useProjects();vi.spyOn(api,'projects').mockResolvedValue([]);await p.load();expect(p.id.value).toBe('')
 await p.loadTrash();expect(p.trash.value).toEqual([])
 p.select('default');vi.spyOn(api,'deletedTickets').mockResolvedValue([{id:'SW-001'} as any]);await p.loadTrash()
 vi.spyOn(api,'restoreTicket').mockResolvedValue({id:'SW-001'} as any);await p.restoreTicket('SW-001');expect(p.trash.value).toEqual([])
 vi.spyOn(api,'createProject').mockRejectedValue(new Error('offline'));expect(await p.create({name:'New',description:''})).toBe(false);expect(p.error.value).toBe('offline')
})
it('uses scoped project and ticket API contracts',async()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,json:async()=>[]})))
 await api.project('second');await api.saveProject({name:'Second',description:''},'second');await api.updateProject('second',{name:'Second',description:''})
 await api.create({title:'Task',prompt:'Prompt'} as any,'second');expect(fetch).toHaveBeenLastCalledWith('/api/tickets?project_id=second',expect.objectContaining({method:'POST'}))
 await api.groups('second');await api.saveGroup('',{name:'Group',color:'blue'},'second')
 await api.deleteProject('second');await api.restoreProject('second');await api.deleteTicket('SW-001');await api.restoreTicket('SW-001')
 expect(fetch).toHaveBeenLastCalledWith('/api/tickets/SW-001/restore',expect.objectContaining({method:'POST'}))
})
it('ignores an old project response after switching boards',async()=>{
 const id=ref('default'),w=useWorkspace(id)
 let resolve!:(value:any)=>void
 vi.spyOn(api,'list').mockImplementationOnce(()=>new Promise(done=>{resolve=done})).mockResolvedValue([])
 vi.spyOn(api,'groups').mockResolvedValue([])
 const old=w.refresh();id.value='second';w.reset();await w.refresh()
 resolve([{id:'SW-001',title:'Old project'}]);await old
 expect(w.tickets.value).toEqual([])
})
it('ignores an old project failure and deleted-ticket list after switching',async()=>{
 const id=ref('default'),w=useWorkspace(id)
 let reject!:(value:any)=>void
 vi.spyOn(api,'list').mockImplementationOnce(()=>new Promise((_,fail)=>{reject=fail})).mockResolvedValue([])
 vi.spyOn(api,'groups').mockResolvedValue([])
 const old=w.refresh();id.value='second';w.reset();await w.refresh();reject(new Error('Old connection failed'));await old
 expect(w.online.value).toBe(true)
 const p=useProjects();p.select('default')
 let resolve!:(value:any)=>void
 vi.spyOn(api,'deletedTickets').mockImplementationOnce(()=>new Promise(done=>{resolve=done})).mockResolvedValue([])
 const oldTrash=p.loadTrash();p.select('second');await p.loadTrash();resolve([{id:'SW-001',title:'Other project'}]);await oldTrash
 expect(p.trash.value).toEqual([])
})
