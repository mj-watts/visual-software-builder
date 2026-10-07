<script setup lang="ts">
import { ref } from 'vue'
import type { ManagedProject, Project, Ticket } from '../types'
import PreviewDialog from './PreviewDialog.vue'
import ProjectDialog from './ProjectDialog.vue'
defineProps<{open:boolean;page?:boolean;projects:ManagedProject[];selectedId:string;trash:Ticket[];busy:boolean;error:string}>()
const emit=defineEmits<{'update:open':[boolean];select:[string];save:[string,Project];remove:[string];restore:[string];restoreTicket:[string]}>()
const editing=ref(false),editId=ref(''),fields=ref<Project>({name:'',description:''})
function edit(project?:ManagedProject) {editId.value=project?.id ?? '';fields.value={name:project?.name ?? '',description:project?.description ?? '',...(project?.screenshot!==undefined?{screenshot:project.screenshot}:{})};editing.value=true}
function save(value:Project) {emit('save',editId.value,value)}
defineExpose({saved:()=>{editing.value=false}})
</script>
<template>
 <component :is="page?'main':PreviewDialog" v-if="open" :class="{'projects-page':page}" :open="open" title="Manage projects" description="Give each project its own board. Deleted projects and tickets can be restored here." @update:open="emit('update:open',$event)">
  <header v-if="page" class="projects-page-heading"><div><div class="eyebrow"><span class="green-dot"/>YOUR WORKSPACE</div><h2>Projects</h2><p>A place for every idea. Open a board to pick up where you left off.</p></div><button class="secondary" @click="emit('update:open',false)">Back to board</button></header>
  <div class="project-manager"><button class="primary" @click="edit()">New project</button><p v-if="error" role="alert" class="error">{{error}}</p>
   <h3>Projects</h3><p v-if="!projects.some(p=>!p.deleted)" class="small-note">No projects yet. Create one or restore a deleted project.</p>
   <div class="project-grid"><article v-for="project in projects.filter(p=>!p.deleted)" :key="project.id" class="project-row project-card" :data-project="project.id"><button class="project-cover" :aria-label="`Open ${project.name} board`" :disabled="busy" @click="emit('select',project.id)"><img v-if="project.screenshot" :src="project.screenshot" :alt="`${project.name} screenshot`"/><span v-else class="project-cover-empty">{{project.name.charAt(0).toUpperCase()}}</span></button><div><strong>{{project.name}}</strong><span v-if="project.id===selectedId" class="status-pill">Current</span><p>{{project.description || 'No description yet'}}</p></div><div class="project-actions"><button class="secondary" :disabled="busy || (!page && project.id===selectedId)" @click="emit('select',project.id)">Open {{project.name}}</button><button class="secondary" :disabled="busy" @click="edit(project)">Edit {{project.name}}</button><button class="danger-button" :disabled="busy" @click="emit('remove',project.id)">Delete {{project.name}}</button></div></article></div>
   <h3>Deleted projects</h3><p v-if="!projects.some(p=>p.deleted)" class="small-note">No deleted projects.</p><div v-for="project in projects.filter(p=>p.deleted)" :key="project.id" class="project-row"><strong>{{project.name}}</strong><button class="secondary" :disabled="busy" @click="emit('restore',project.id)">Restore {{project.name}}</button></div>
   <h3>Deleted tickets in the current project</h3><p v-if="!trash.length" class="small-note">No deleted tickets.</p><div v-for="ticket in trash" :key="ticket.id" class="project-row"><div><strong>{{ticket.title}}</strong><p>{{ticket.id}}</p></div><button class="secondary" :disabled="busy" @click="emit('restoreTicket',ticket.id)">Restore {{ticket.title}}</button></div>
  </div>
 </component>
 <ProjectDialog v-model:open="editing" :project="fields" :creating="!editId" :busy="busy" :error="error" @save="save"/>
</template>
