<script setup lang="ts">
import { GripVertical, ArrowUpRight, Image, FileCode2, LoaderCircle, AlertCircle, Check } from 'lucide-vue-next'
import type { Ticket } from '../types'
import { matchesStatus } from '../domain'
defineProps<{ticket:Ticket;selectedId?:string;selectedIds?:string[]}>()
const emit=defineEmits<{select:[Ticket,MouseEvent]}>()
function drag(event:DragEvent,ticket:Ticket) {event.dataTransfer?.setData('text/plain',ticket.id)}
</script>
<template>
      <button class="ticket" :class="{selected:ticket.id===selectedId,'multi-selected':selectedIds?.includes(ticket.id)}" :aria-pressed="selectedIds?.includes(ticket.id) ?? false" :data-ticket="ticket.id" draggable="true" @dragstart="drag($event,ticket)" @click="emit('select',ticket,$event)" @contextmenu.ctrl.prevent="emit('select',ticket,$event)">
       <span class="ticket-top"><span class="ticket-id">{{ticket.id}}</span><GripVertical v-if="ticket.status==='todo'" class="grip" :size="15"/><span v-else-if="ticket.status==='done'" class="completion-mark" role="img" aria-label="Completed" title="Completed"><Check :size="13" :stroke-width="3" aria-hidden="true"/></span><ArrowUpRight v-else :size="15"/></span>
       <strong>{{ ticket.title }}</strong><p>{{ ticket.prompt }}</p>
       <span class="ticket-footer"><span class="provider-chip"><span class="tiny-logo">✳</span> {{ticket.provider==='demo'?'Demo agent':ticket.provider==='codex'?'Codex':'Claude'}}</span><span v-if="ticket.images.length" class="meta"><Image :size="12"/>{{ticket.images.length}}</span><span v-if="ticket.files.length" class="meta"><FileCode2 :size="12"/>{{ticket.files.length}}</span></span>
       <span v-if="matchesStatus(ticket.status,'active')" class="run-status" :class="ticket.status"><LoaderCircle v-if="ticket.status==='running'" :size="12" class="spin"/><AlertCircle v-if="ticket.status==='failed'" :size="12"/>{{ ticket.status==='queued'?'Queued':ticket.status==='failed'?'Failed':ticket.status==='cancelled'?'Cancelled':'Running' }}</span>
       <span v-if="ticket.status==='queued' || ticket.status==='running'" class="ticket-progress" :class="ticket.status" role="progressbar" :aria-label="`Progress for ${ticket.id}`" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="ticket.status==='queued'?0:undefined" :aria-valuetext="ticket.status==='queued'?'Waiting to start':'Agent is working'"><span class="ticket-progress-fill"/></span>
      </button>
</template>
