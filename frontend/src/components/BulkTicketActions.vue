<script setup lang="ts">
import { computed, ref } from 'vue'
import { Trash2, X } from 'lucide-vue-next'
import type { Ticket } from '../types'
import PreviewDialog from './PreviewDialog.vue'
const props=defineProps<{tickets:Ticket[];busy:boolean;error:string;unsaved:boolean}>()
const emit=defineEmits<{clear:[];remove:[string[]]}>()
const confirming=ref(false)
const blocked=computed(()=>props.tickets.some(t=>['queued','running'].includes(t.status)))
const limit=computed(()=>props.tickets.length>200)
</script>
<template>
 <aside class="bulk-ticket-actions" role="toolbar" aria-label="Selected ticket actions">
  <span role="status" aria-live="polite">{{tickets.length}} {{tickets.length===1?'ticket':'tickets'}} selected</span>
  <button class="danger-button" :disabled="busy || blocked || limit" @click="confirming=true"><Trash2 :size="15"/>Delete all</button>
  <button class="icon-button" aria-label="Clear ticket selection" :disabled="busy" @click="emit('clear')"><X :size="18"/></button>
  <p v-if="blocked">Cancel or finish queued/running tickets before deleting.</p><p v-if="limit">Select up to 200 tickets at a time.</p>
 </aside>
 <PreviewDialog v-model:open="confirming" :title="`Delete ${tickets.length} selected ${tickets.length===1?'ticket':'tickets'}?`" description="These tickets will leave the board. You can restore them from Deleted tickets on the projects page.">
  <div class="bulk-delete-confirmation"><p v-if="unsaved" class="error">Unsaved edits to a selected ticket will be discarded.</p><ul><li v-for="ticket in tickets" :key="ticket.id">{{ticket.id}} · {{ticket.title}}</li></ul><p v-if="error" role="alert" class="error">{{error}}</p><p v-if="blocked" role="alert">A selected ticket is queued or running. Cancel or finish it before deleting.</p><div class="project-actions"><button class="secondary" :disabled="busy" @click="confirming=false">Cancel</button><button class="danger-button" :disabled="busy || blocked || limit" @click="emit('remove',tickets.map(t=>t.id))">{{busy?'Deleting…':'Confirm delete all'}}</button></div></div>
 </PreviewDialog>
</template>
