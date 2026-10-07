import { it,expect,vi } from 'vitest'
import { mount,flushPromises } from '@vue/test-utils'
import { useProject } from '../src/project'
import { api } from '../src/api'
import ProjectDialog from '../src/components/ProjectDialog.vue'
import StatusFilter from '../src/components/StatusFilter.vue'
const project={name:'Swimlane',description:'Build through prompts'}
it('loads and saves project fields; reports failures without overwriting them',async()=>{
 vi.spyOn(api,'project').mockResolvedValue(project);vi.spyOn(api,'saveProject').mockResolvedValue({...project,description:'Updated'})
 const p=useProject();await p.load();expect(p.project.value).toEqual(project);expect(await p.save({...project,description:'Updated'})).toBe(true);expect(p.project.value.description).toBe('Updated')
 vi.mocked(api.saveProject).mockRejectedValue(new Error('offline'));expect(await p.save(project)).toBe(false);expect(p.error.value).toBe('offline');expect(p.project.value.description).toBe('Updated')
 vi.mocked(api.project).mockRejectedValue('offline');await p.load();expect(p.error.value).toContain('load');vi.restoreAllMocks()
})
it('edits project name and description in an accessible dialog',async()=>{
 const w=mount(ProjectDialog,{props:{open:false,project,busy:false,error:''}});await w.setProps({open:true});await flushPromises()
 const title=document.querySelector('#project-name') as HTMLInputElement;expect(title.value).toBe('Swimlane');title.value='New name';title.dispatchEvent(new Event('input',{bubbles:true}))
 const description=document.querySelector('#project-description') as HTMLTextAreaElement;description.value='New description';description.dispatchEvent(new Event('input',{bubbles:true}));await flushPromises()
 document.querySelector('.project-form')!.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));expect(w.emitted('save')?.[0]).toEqual([{name:'New name',description:'New description'}]);w.unmount()
})
it('opens the styled status menu and selects a status',async()=>{
 const w=mount(StatusFilter,{props:{modelValue:'all'}})
 await w.get('[aria-label="Filter status"]').trigger('click',{button:0,ctrlKey:false});await flushPromises()
 const item=document.querySelector('[data-status="done"]') as HTMLElement;expect(item).not.toBeNull();item.click();await flushPromises();expect(w.emitted('update:modelValue')?.[0]).toEqual(['done']);w.unmount()
})
it('uses the project API contract',async()=>{
 vi.stubGlobal('fetch',vi.fn().mockResolvedValue({ok:true,json:async()=>project}));await api.project();await api.saveProject(project)
 expect(fetch).toHaveBeenLastCalledWith('/api/project',expect.objectContaining({method:'PUT',body:JSON.stringify(project)}));vi.unstubAllGlobals()
})
