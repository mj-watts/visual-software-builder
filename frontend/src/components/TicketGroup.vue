<script setup lang="ts">
import { ref } from 'vue'
import { Minus, Plus, Ellipsis } from 'lucide-vue-next'
import { groupAppearance } from '../groupColors'
import type { Group, Ticket } from '../types'
import TicketCard from './TicketCard.vue'
defineProps<{group:Group;lane:string;tickets:Ticket[];selectedId?:string;selectedIds?:string[];allowCreate:boolean}>()
const emit=defineEmits<{select:[Ticket,MouseEvent];create:[string];edit:[Group];remove:[string];run:[string];assign:[string,string]}>()
const collapsed=ref(false)
const over=ref(false)
function drop(event:DragEvent,groupId:string) {over.value=false;emit('assign',event.dataTransfer?.getData('text/plain') ?? '',groupId)}
function closeMenu(event:MouseEvent) {(event.currentTarget as HTMLElement).closest('details')?.removeAttribute('open')}
</script>
<template>
 <section class="ticket-group" :class="{ 'group-dropzone':over }" :style="groupAppearance(group.color)" :data-group="group.id" :aria-label="`${group.name} group in ${lane}`" @dragover.stop.prevent="over=true" @dragleave.stop="over=false" @drop.stop.prevent="drop($event,group.id)">
  <header class="group-header"><strong>{{group.name}}</strong><span class="group-count">{{tickets.length}}</span><button class="group-control" :title="collapsed?'Expand group':'Minimise group'" :aria-label="`${collapsed?'Expand':'Minimise'} ${group.name} in ${lane}`" :aria-expanded="!collapsed" @click="collapsed=!collapsed"><Plus v-if="collapsed" :size="14" aria-hidden="true"/><Minus v-else :size="14" aria-hidden="true"/></button><details class="group-menu"><summary class="group-control" title="Group menu" :aria-label="`Menu for ${group.name} in ${lane}`"><Ellipsis :size="14" aria-hidden="true"/></summary><div class="menu-surface" @click="closeMenu"><button v-if="allowCreate" :disabled="!tickets.length" :aria-label="`Run ${group.name} group`" @click="$emit('run',group.id)">Run group</button><small v-if="allowCreate">Saved Todo prompts · top to bottom. Stops on failure.</small><button :aria-label="`Edit ${group.name}`" @click="$emit('edit',group)">Edit group</button><button :aria-label="`Remove ${group.name}`" @click="$emit('remove',group.id)">Remove group</button><small>Tickets will be kept.</small></div></details></header>
  <div v-if="!collapsed" class="group-cards"><button v-if="allowCreate" class="add-ticket" :aria-label="`Add ticket to ${group.name}`" @click="$emit('create',group.id)">+ Add ticket</button><TicketCard v-for="ticket in tickets" :key="ticket.id" :ticket="ticket" :selected-id="selectedId" :selected-ids="selectedIds" @select="(ticket,event)=>emit('select',ticket,event)"/></div>
 </section>
</template>
