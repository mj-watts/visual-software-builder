<script setup lang="ts">
import { ref } from 'vue'
import { History, FileCode2 } from 'lucide-vue-next'
import type { Run, RunEvent, ChangedFile } from '../types'
import { diffClass } from '../domain'
import { api } from '../api'
import PreviewDialog from './PreviewDialog.vue'
defineProps<{runs:Run[];events:RunEvent[];live:boolean}>()
const selected=ref<Run>(),file=ref<ChangedFile>(),activity=ref<RunEvent[]>([])
async function inspect(run:Run) {selected.value=run;file.value=undefined;activity.value=await api.events(run.id).catch(()=>[])}
</script>
<template>
 <section v-if="runs.length" class="run-history"><div class="section-label"><label><History :size="15"/>Run history</label><span class="small-note">{{live?'Live':'Polling'}}</span></div>
  <button v-for="(run,index) in runs" :key="run.id" class="history-row" @click="inspect(run)"><span>Run {{runs.length-index}} · {{run.snapshot.provider}}</span><span class="status-pill" :class="run.status">{{run.status}}</span></button>
  <div v-if="events.length" class="run-activity" aria-label="Run activity"><p v-for="event in events.slice(-8)" :key="event.id"><span>{{event.type}}</span>{{event.message}}</p></div>
 </section>
 <PreviewDialog :open="!!selected" title="Run review" description="Saved prompt, results and activity for this attempt." @update:open="selected=undefined">
  <div v-if="selected" class="run-review"><div class="task-heading"><span class="status-pill" :class="selected.status">{{selected.status}}</span><span class="small-note">{{selected.created_at}}</span></div><h3>{{selected.snapshot.title}}</h3><p class="response-text">{{selected.snapshot.prompt}}</p><p class="small-note">{{selected.snapshot.provider}} · {{selected.snapshot.permission}} · Tests: {{selected.tests}}</p><p v-if="selected.base_commit" class="small-note">Base commit: <code>{{selected.base_commit}}</code></p><p v-if="selected.worktree" class="worktree-path">Retained worktree <code>{{selected.worktree}}</code></p><p class="response-text">{{selected.response || 'Waiting for a result…'}}</p>
   <button v-for="changed in selected.files" :key="changed.path" class="file-row" @click="file=changed"><FileCode2 :size="15"/><span>{{changed.path}}</span><b class="green">+{{changed.additions}}</b><b class="red">−{{changed.deletions}}</b></button>
   <pre v-if="file" class="diff"><code v-for="(line,index) in file.diff.split('\n')" :key="index" :class="diffClass(line)">{{line}}<br></code></pre>
   <h3>Activity</h3><div class="run-activity"><p v-for="event in activity" :key="event.id"><span>{{event.type}}</span>{{event.message}}</p></div>
  </div>
 </PreviewDialog>
</template>
