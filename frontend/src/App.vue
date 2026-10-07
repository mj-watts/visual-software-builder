<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { Layers3, LayoutDashboard, Plus, Search, Pencil, ArrowUpRight, CircleHelp, Workflow, Command, ArrowRight } from 'lucide-vue-next'
import ProjectSwitch from './components/ProjectSwitch.vue'
import StatusFilter from './components/StatusFilter.vue'
import ProjectDialog from './components/ProjectDialog.vue'
import { useProject } from './project'
import { useProjects } from './projects'
import ProjectManager from './components/ProjectManager.vue'
import GroupDialog from './components/GroupDialog.vue'
import type { Group, Project } from './types'
import Board from './components/Board.vue'
import TaskPane from './components/TaskPane.vue'
import PreviewDialog from './components/PreviewDialog.vue'
import { useWorkspace } from './workspace'
import { useExecution } from './execution'
import WorkspaceSettings from './components/WorkspaceSettings.vue'
import { visibleTickets } from './domain'
import { useTicketSelection } from './ticketSelection'
import BulkTicketActions from './components/BulkTicketActions.vue'
const projects=useProjects(),w=useWorkspace(projects.id)
const selection=useTicketSelection()
function pickTicket(ticket:import('./types').Ticket,event:MouseEvent,order:string[]) {if(w.busy.value)return;if(selection.click(ticket.id,event,order))w.select(ticket)}
function newTicket(groupId='') {selection.clear();w.newTicket(groupId)}
async function deleteSelection(ids:string[]) {if(await w.removeTickets(ids))selection.clear()}
const selectedTickets=computed(()=>w.tickets.value.filter(ticket=>selection.ids.value.includes(ticket.id)))
watch(()=>w.tickets.value.map(t=>t.id),ids=>selection.retain(ids))
const project=useProject(projects.id),projectOpen=ref(false),managerOpen=ref(false),discardOpen=ref(false),manager=ref<InstanceType<typeof ProjectManager>>()
const pendingProject=ref('')
async function saveProject(value:Project) {if(await project.save(value)){projectOpen.value=false;await projects.load()}}
async function manageSave(id:string,value:Project) {const saved=id?await projects.update(id,value):await projects.create(value);if(saved){manager.value?.saved();await project.load();await w.refresh();await projects.loadTrash()}}
function hasUnsavedTicket() {return w.editable.value && w.dirty.value && (w.selected.value || w.creating.value)}
function openProjects() {pendingProject.value='';if(hasUnsavedTicket()){discardOpen.value=true;return};selection.clear();managerOpen.value=true}
function discardAndManage() {selection.clear();w.reset();discardOpen.value=false;if(pendingProject.value){projects.select(pendingProject.value);pendingProject.value='';managerOpen.value=false}else managerOpen.value=true}
function selectProject(id:string) {if(id!==projects.id.value && hasUnsavedTicket()){pendingProject.value=id;discardOpen.value=true;return};selection.clear();projects.select(id);managerOpen.value=false}
watch(managerOpen,async open=>{if(open){await projects.load();await projects.loadTrash()}})
watch(projects.id,async()=>{selection.clear();w.reset();query.value='';filter.value='all';await project.load();await w.refresh();if(managerOpen.value)await projects.loadTrash()})
async function restoreTicket(id:string) {if(await projects.restoreTicket(id))await w.refresh()}
const groupOpen=ref(false),editingGroup=ref<Group>()
function openGroup(group?:Group) {editingGroup.value=group;w.error.value='';groupOpen.value=true}
async function saveGroup(id:string,value:Omit<Group,'id'>) {if(await w.saveGroup(id,value))groupOpen.value=false}
const settingsOpen=ref(false)
const execution=useExecution(async()=>{await w.refresh();await execution.history(w.selected.value?.id ?? '')})
watch(()=>w.selected.value?.id, id=>execution.history(id ?? ''))
watch(()=>w.selected.value?.status, ()=>execution.history(w.selected.value?.id ?? ''))
const query=ref(''),filter=ref('all'),help=ref(false)
watch([query,filter],()=>selection.clear())
const shown=computed(()=>visibleTickets(w.tickets.value,query.value,filter.value))
const active=computed(()=>w.tickets.value.filter(t=>['queued','running'].includes(t.status)).length)
onMounted(async()=>{await projects.load();project.load();w.refresh();execution.load();execution.start()})
onUnmounted(()=>execution.stop())
</script>
<template>
 <div class="app-shell">
  <nav class="sidebar" aria-label="Workspace navigation"><button class="brand" aria-label="Open projects" title="Projects" :disabled="w.busy.value" @click="openProjects"><Layers3 :size="22"/></button><div class="nav-divider"/><button class="nav-item" :class="{active:!managerOpen}" aria-label="Board" @click="managerOpen=false"><LayoutDashboard :size="20"/></button><button class="nav-item" aria-label="How it works" @click="help=true"><Workflow :size="20"/></button><div class="sidebar-bottom"><button class="nav-item" aria-label="Help" @click="help=true"><CircleHelp :size="20"/></button><span class="avatar">MW</span></div></nav>
  <div class="workspace-shell">
   <header class="topbar"><div class="breadcrumb"><button class="workspace-link" :disabled="w.busy.value" @click="openProjects">Workspace</button> <span>/</span><ProjectSwitch :projects="projects.items.value" :selected-id="projects.id.value" :label="managerOpen?'Projects':project.project.value.name" :busy="w.busy.value || projects.busy.value" @select="selectProject" @manage="openProjects"/><span class="local-label">LOCAL</span></div><button class="top-help" aria-label="Repository settings" @click="settingsOpen=true">Repositories <Layers3 :size="14"/></button><button class="top-help" @click="help=true">How it works <ArrowUpRight :size="14"/></button></header>
   <ProjectManager ref="manager" page v-model:open="managerOpen" :projects="projects.items.value" :selected-id="projects.id.value" :trash="projects.trash.value" :busy="projects.busy.value" :error="projects.error.value" @select="selectProject" @save="manageSave" @remove="projects.remove" @restore="projects.restore" @restore-ticket="restoreTicket"/>
   <div v-show="!managerOpen" class="workspace-main">
    <main class="board-main">
     <div class="board-title"><div class="project-heading"><div class="eyebrow"><span class="green-dot"/>LOCAL PROJECT</div><div class="project-title-row"><h1>{{project.project.value.name}}</h1><button class="icon-button" aria-label="Edit project" title="Edit project" :disabled="!projects.id.value" @click="projectOpen=true"><Pencil :size="16"/></button></div><p v-if="project.project.value.description" class="project-description">{{project.project.value.description}}</p><button v-else class="project-description-add" @click="projectOpen=true">Add a project description</button></div><button class="primary" data-testid="new-ticket" :disabled="!projects.id.value" @click="newTicket()"><Plus :size="17"/>New ticket</button></div>
     <div class="board-toolbar"><div class="view-tabs"><span class="view-tab"><LayoutDashboard :size="15"/>Board</span><span class="total-count">{{w.tickets.value.length}} tickets</span></div><div class="board-controls"><label class="search"><Search :size="15"/><input v-model="query" aria-label="Search tickets" placeholder="Search tickets…"/></label><StatusFilter v-model="filter"/></div></div>
     <p v-if="!w.online.value" role="alert" class="connection-error">Backend unavailable. Start the backend, then <button @click="w.refresh">try again</button>.</p>
     <p v-if="w.error.value && !w.selected.value && !w.creating.value && !groupOpen" role="alert" class="error">{{w.error.value}}</p>
     <p v-if="w.loading.value" class="loading-message">Opening your workspace…</p>
     <Board v-if="projects.id.value" :groups="w.groups.value" :tickets="shown" :selected-id="w.selected.value?.id" :selected-ids="selection.ids.value" :loading="w.loading.value" @select="pickTicket" @create="newTicket" @drop="w.drop" @create-group="openGroup()" @edit-group="openGroup" @delete-group="w.deleteGroup" @run-group="w.runGroup" @assign-group="w.moveToGroup"/>
     <footer class="board-footer"><span><span class="green-dot"/>{{w.online.value?'Local workspace connected':'Reconnecting to workspace'}}</span><span>{{active?`${active} ticket${active===1?'':'s'} in the queue`:'One idea at a time. Built at your pace.'}}</span></footer>
    </main>
    <TaskPane v-if="projects.id.value" @delete="w.removeTicket(w.selectedId.value)" :groups="w.groups.value" @assign-group="w.assignGroup" :ticket="w.selected.value" :draft="w.draft.value" :creating="w.creating.value" :editable="w.editable.value" :busy="w.busy.value" :dirty="w.dirty.value" :error="w.error.value" :providers="execution.providers.value" :workspaces="execution.workspaces.value" :runs="execution.runs.value" :events="execution.events.value" :live="execution.live.value" @settings="settingsOpen=true" @cancel="w.cancel" @retry="w.retry" @save="w.save" @run="w.run" @paste="w.paste" @create="newTicket()"/>
   </div>
   <div class="demo-strip"><span class="demo-tag">{{execution.providers.value.some(p=>p.id!=='demo' && p.available)?'LOCAL AGENTS':'DEMO MODE'}}</span><span>Real agents work in isolated Git worktrees. Demo tickets stay simulated.</span><button @click="help=true">Meet your workspace <ArrowRight :size="13"/></button></div>
  </div>
  <BulkTicketActions v-if="selection.ids.value.length && !managerOpen" :tickets="selectedTickets" :busy="w.busy.value" :error="w.error.value" :unsaved="!!w.selected.value && selection.ids.value.includes(w.selectedId.value) && w.editable.value && w.dirty.value" @clear="selection.clear" @remove="deleteSelection"/>
  <PreviewDialog v-model:open="discardOpen" title="Discard unsaved ticket changes?" description="Continue and discard your unsaved ticket edits."><div class="project-manager"><button class="secondary" @click="discardOpen=false">Keep editing</button><button class="danger-button" @click="discardAndManage">Discard changes</button></div></PreviewDialog>
  <ProjectDialog v-model:open="projectOpen" :project="project.project.value" :busy="project.busy.value" :error="project.error.value" @save="saveProject"/>
  <GroupDialog v-model:open="groupOpen" :group="editingGroup" :busy="w.busy.value" :error="w.error.value" @save="saveGroup"/>
  <WorkspaceSettings v-model:open="settingsOpen" :workspaces="execution.workspaces.value" :providers="execution.providers.value" :roots="execution.roots.value" :timeout="execution.timeout.value" :error="execution.error.value" :busy="execution.busy.value" @register="execution.register"/>
  <PreviewDialog v-model:open="help" title="A ticket is your starting point." description="Build at the speed of an idea."><div class="help-body"><p><b>1. Shape your prompt.</b> Create a ticket, add context and paste reference images directly into the prompt.</p><p><b>2. Give it to your agent.</b> Drag a saved Todo ticket into Doing, or use Run ticket. Runs wait in a single queue.</p><p><b>3. Review the result.</b> Completed tickets move to Done. Select a changed file to inspect its diff.</p><div class="help-note">Choose Demo for simulated runs, or configure backend credentials and a Git repository for Codex or Claude. Real runs use committed HEAD and retain changes in an isolated worktree for review. Cancel a run or retry a failed attempt from its ticket.</div></div></PreviewDialog>
 </div>
</template>
