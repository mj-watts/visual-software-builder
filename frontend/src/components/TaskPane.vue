<script setup lang="ts">
import { ref } from 'vue'
import { Trash2, Sparkles, ArrowRight, ImagePlus, FileCode2, ChevronDown, ChevronRight, Check, X, Play, Save, MessageSquare } from 'lucide-vue-next'
import type { Draft, Ticket, ChangedFile, ProviderInfo, Workspace, Run, RunEvent, Group } from '../types'
import { diffClass } from '../domain'
import PreviewDialog from './PreviewDialog.vue'
import RunHistory from './RunHistory.vue'
withDefaults(defineProps<{groups?:Group[];ticket?:Ticket;draft:Draft;creating:boolean;editable:boolean;busy:boolean;dirty:boolean;error:string;providers:ProviderInfo[];workspaces:Workspace[];runs:Run[];events:RunEvent[];live:boolean}>(),{groups:()=>[]})
defineEmits<{save:[];run:[];paste:[ClipboardEvent];create:[];settings:[];cancel:[];retry:[];assignGroup:[string];delete:[]}>()
const image=ref(''),file=ref<ChangedFile>()
function removeImage(draft:Draft,index:number) {draft.images.splice(index,1)}
</script>
<template>
 <aside class="task-pane" aria-label="Ticket details">
  <header class="pane-header"><div><span class="eyebrow">YOUR WORK, IN FOCUS</span><h2>{{creating?'New ticket':'Ticket details'}}</h2></div><div class="pane-header-actions"><span class="ticket-id">{{ticket?.id ?? 'DRAFT'}}</span><button v-if="ticket && !creating" class="icon-button" aria-label="Delete ticket" :disabled="busy || ticket.status==='queued' || ticket.status==='running'" :title="ticket.status==='queued' || ticket.status==='running'?'Cancel the run before deleting':'Move ticket to Deleted'" @click="$emit('delete')"><Trash2 :size="16"/></button></div></header>
  <div v-if="!ticket && !creating" class="inspector-empty"><Sparkles :size="32"/><h3>Every idea starts here.</h3><p>Select a ticket to shape your prompt,<br>follow the work and review changes.</p><button class="secondary" @click="$emit('create')">Create your first ticket <ArrowRight :size="15"/></button></div>
  <template v-else>
   <div class="pane-scroll">
    <div class="task-heading"><span class="status-pill" :class="ticket?.status ?? 'todo'">{{creating?'Draft':ticket?.status==='todo'?'Todo':ticket?.status==='done'?'Done':ticket?.status==='queued'?'Queued':ticket?.status==='failed'?'Failed':ticket?.status==='cancelled'?'Cancelled':'Running'}}</span><span class="small-note">{{dirty && editable?'Unsaved changes': 'Local workspace'}}</span></div>
    <input v-if="editable" v-model="draft.title" class="title-input" aria-label="Ticket title" maxlength="160" placeholder="Give your idea a name…"/>
    <h3 v-else class="task-title">{{ticket?.title}}</h3>
    <section class="ticket-group-field"><label for="ticket-group">Group</label><div class="group-select"><select v-if="editable" id="ticket-group" v-model="draft.group_id"><option value="">No group</option><option v-for="group in groups" :key="group.id" :value="group.id">{{group.name}}</option></select><select v-else id="ticket-group" :value="ticket?.group_id ?? ''" :disabled="busy" @change="$emit('assignGroup',($event.target as HTMLSelectElement).value)"><option value="">No group</option><option v-for="group in groups" :key="group.id" :value="group.id">{{group.name}}</option></select><ChevronDown :size="14" aria-hidden="true"/></div></section>
    <section class="prompt-section"><div class="section-label"><label for="prompt"><Sparkles :size="15"/>Prompt</label><span class="small-note">{{draft.prompt.length.toLocaleString()}} / 20k</span></div>
     <textarea id="prompt" v-model="draft.prompt" :readonly="!editable" maxlength="20000" placeholder="Describe what you want to build. A little context goes a long way…" @paste="$emit('paste',$event)"/>
     <div class="attachment-hint"><ImagePlus :size="14"/><span>{{editable?'Paste images into your prompt':'Reference images'}} · PNG, JPG, WebP</span></div>
     <div v-if="draft.images.length" class="attachments"><div v-for="(src,index) in draft.images" :key="src" class="attachment"><button aria-label="Preview attached image" @click="image=src"><img :src="src" alt="Prompt reference"/></button><button v-if="editable" class="remove-image" aria-label="Remove image" @click="removeImage(draft,index)"><X :size="12"/></button></div></div>
    </section>
    <section class="agent-section"><label for="agent">Agent</label><div class="agent-select"><span class="tiny-logo">✳</span><select id="agent" v-model="draft.provider" :disabled="!editable"><option v-for="provider in providers" :key="provider.id" :value="provider.id" :disabled="!provider.available">{{provider.name}}{{provider.available?'':' · Not connected'}}</option></select><span class="demo-badge">{{draft.provider==='demo'?'DEMO':'LOCAL AGENT'}}</span></div><p class="small-note">{{draft.provider==='demo'?'Simulated runs. No API key needed.':'Server credentials · isolated worktree'}}</p></section>
    <section v-if="draft.provider!=='demo'" class="execution-settings">
     <label for="workspace">Repository</label><select id="workspace" v-model="draft.workspace_id" :disabled="!editable"><option value="">Select a Git repository…</option><option v-for="workspace in workspaces" :key="workspace.id" :value="workspace.id">{{workspace.name}}</option></select><button class="text-button" @click="$emit('settings')">Manage repositories</button>
     <label for="permission">Execution permission</label><select id="permission" v-model="draft.permission" :disabled="!editable"><option value="read-only">Read only · review and explain</option><option value="workspace-write">Allow edits in the isolated worktree</option></select>
     <label class="checkbox-label"><input v-model="draft.allow_tests" type="checkbox" :disabled="!editable"/>Run configured tests (executes repository code)</label>
     <p class="small-note">Uses committed HEAD. Source checkout remains unchanged.</p>
    </section>
    <section class="response-section"><div class="section-label"><label><MessageSquare :size="15"/>Response</label><Check v-if="ticket?.status==='done'" :size="14" class="green"/></div><p v-if="ticket?.response" class="response-text">{{ticket.response}}</p><div v-else class="response-placeholder"><span class="response-dot"/><p>{{ticket?.status==='running'?'The demo agent is working…':ticket?.status==='queued'?'Waiting for the current run to finish…':'A little quiet, for now.'}}<span>The agent’s response will appear here.</span></p></div></section>
    <section class="files-section"><div class="section-label"><label><FileCode2 :size="15"/>Files changed</label><span class="count">{{ticket?.files.length ?? 0}}</span></div><template v-if="ticket?.files.length"><p class="small-note">{{draft.provider==='demo'?'Illustrative demo diffs · click to review':'Actual Git changes · source checkout unchanged'}}</p><button v-for="changed in ticket.files" :key="changed.path" class="file-row" @click="file=changed"><FileCode2 :size="15"/><span>{{changed.path}}</span><b class="green">+{{changed.additions}}</b><b class="red">−{{changed.deletions}}</b><ChevronRight :size="14"/></button></template><p v-else class="files-placeholder">Changed files will be listed after a run.</p></section>
    <RunHistory :runs="runs" :events="events" :live="live"/>
   </div>
   <footer class="pane-actions"><p v-if="error" role="alert" class="error">{{error}}</p><template v-if="editable"><button class="secondary" data-testid="save-ticket" :disabled="busy" @click="$emit('save')"><Save :size="15"/>{{creating?'Create ticket':'Save changes'}}</button><button v-if="!creating" class="primary" data-testid="run-ticket" :disabled="busy" @click="$emit('run')"><Play :size="14"/>Run ticket</button></template><button v-else-if="ticket?.status==='queued' || ticket?.status==='running'" class="secondary" :disabled="busy" @click="$emit('cancel')">Cancel run</button><button v-else-if="ticket?.status==='failed' || ticket?.status==='cancelled'" class="primary" :disabled="busy" @click="$emit('retry')">Retry ticket</button><span v-else class="small-note"><Check :size="14"/>{{ticket?.status==='done'?'Run complete. Changes ready to review.':'Prompt locked for this run.'}}</span></footer>
  </template>
  <PreviewDialog :open="!!image" title="Prompt reference" description="Image attached to this ticket" @update:open="image=''"><img class="preview-image" :src="image" alt="Full size prompt reference"/></PreviewDialog>
  <PreviewDialog :open="!!file" :title="file?.path ?? 'File diff'" :description="draft.provider==='demo'?'Illustrative demo diff. No repository files were modified.':'Actual changes in the isolated worktree. Source checkout unchanged.'" @update:open="file=undefined"><pre class="diff"><code v-for="(line,index) in file?.diff.split('\n')" :key="index" :class="diffClass(line)">{{line}}<br></code></pre></PreviewDialog>
 </aside>
</template>
