import { ref, type Ref } from 'vue'
import { api } from './api'
import type { Project } from './types'
export function useProject(id:Ref<string>=ref('default')) {
 const project=ref<Project>({name:'My first project',description:''}),error=ref(''),busy=ref(false)
 async function load() {
  if(!id.value){project.value={name:'No projects yet',description:'Create a project to start building.'};return}
  const selected=id.value
  try {const value=await api.project(selected);if(selected===id.value)project.value=value} catch {error.value='Could not load project details.'}
 }
 async function save(value:Project) {
  busy.value=true;error.value=''
  try {project.value=await api.saveProject(value,id.value);return true}
  catch(e) {error.value=e instanceof Error?e.message:'Could not save project details.';return false}
  finally {busy.value=false}
 }
 return {project,error,busy,load,save}
}
