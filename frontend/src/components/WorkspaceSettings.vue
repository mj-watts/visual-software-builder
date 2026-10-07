<script setup lang="ts">
import { ref } from 'vue'
import type { Workspace, ProviderInfo } from '../types'
import PreviewDialog from './PreviewDialog.vue'
defineProps<{open:boolean;workspaces:Workspace[];providers:ProviderInfo[];roots:string[];timeout:number;error:string;busy:boolean}>()
defineEmits<{ 'update:open':[boolean];register:[Omit<Workspace,'id'>] }>()
const path=ref(''),name=ref(''),preset=ref<Workspace['test_preset']>('none'),directory=ref('.')
</script>
<template>
 <PreviewDialog :open="open" title="Repository settings" description="Connect a local Git repository for real agent runs." @update:open="$emit('update:open',$event)">
 <div class="settings-body">
  <h3>Agents</h3><div v-for="provider in providers" :key="provider.id" class="provider-readiness"><b>{{provider.name}}</b><span :class="provider.available?'green':'small-note'">{{provider.reason}}</span></div>
  <p class="small-note">API keys are configured on the backend. Run timeout: {{timeout}} seconds.</p>
  <h3>Registered repositories</h3><p v-if="!workspaces.length" class="small-note">No repository selected yet.</p><div v-for="workspace in workspaces" :key="workspace.id" class="registered-repo"><b>{{workspace.name}}</b><code>{{workspace.path}}</code><span class="small-note">Tests: {{workspace.test_preset}} · {{workspace.test_directory}}</span></div>
  <form @submit.prevent="$emit('register',{path,name,test_preset:preset,test_directory:directory})">
   <h3>Add or update a repository</h3>
   <label>Project name<input v-model="name" aria-label="Repository name" required maxlength="120" placeholder="My project"/></label>
   <label>Absolute repository path<input v-model="path" aria-label="Repository path" required placeholder="/Users/you/Code/my-project"/></label>
   <p class="small-note">Allowed server roots: {{roots.join(', ') || 'Loading…'}}. Change SWIMLANE_WORKSPACE_ROOTS on the backend to add another root.</p>
   <div class="test-settings"><label>Test preset<select v-model="preset" aria-label="Test preset"><option value="none">No tests</option><option value="vitest">Vitest · npm test -- --run</option><option value="pytest">Pytest · python3 -m pytest -q</option></select></label><label>Test directory<input v-model="directory" aria-label="Test directory" placeholder="frontend"/></label></div>
   <div class="help-note">Runs start from committed HEAD. Unsaved source changes and local dependencies are excluded. Worktrees are retained for review; the source checkout is unchanged. Tests run trusted repository code only when enabled on the ticket.</div>
   <p v-if="error" role="alert" class="error">{{error}}</p><button class="primary" type="submit" :disabled="busy">Register repository</button>
  </form>
 </div>
 </PreviewDialog>
</template>
