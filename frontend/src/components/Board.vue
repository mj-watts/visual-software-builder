<script setup lang="ts">
import { ref } from 'vue'
import { Plus, Circle, LoaderCircle, Check } from 'lucide-vue-next'
import TicketCard from './TicketCard.vue'
import TicketGroup from './TicketGroup.vue'
import type { Ticket, Group } from '../types'
import { matchesStatus } from '../domain'
const props=withDefaults(defineProps<{tickets:Ticket[];groups?:Group[];selectedId?:string;selectedIds?:string[];loading:boolean}>(),{groups:()=>[]})
const emit=defineEmits<{select:[Ticket,MouseEvent,string[]];create:[groupId?:string];drop:[string];createGroup:[];editGroup:[Group];deleteGroup:[string];runGroup:[string];assignGroup:[string,string]}>()
const over=ref(false)
const lanes=[{id:'todo',title:'Todo',subtitle:'Ideas ready to take shape',icon:Circle},{id:'active',title:'Doing',subtitle:'Your agent is on it',icon:LoaderCircle},{id:'done',title:'Done',subtitle:'Ready for a closer look',icon:Check}]
function laneTickets(lane:string) {return props.tickets.filter(t=>matchesStatus(t.status,lane))}
function laneGroups(lane:string) {return props.groups.filter(g=>lane==='todo' || laneTickets(lane).some(t=>t.group_id===g.id))}
function drop(event:DragEvent,lane:string) {
 over.value=false
 const id=event.dataTransfer?.getData('text/plain') ?? ''
 const ticket=props.tickets.find(t=>t.id===id)
 if(ticket && matchesStatus(ticket.status,lane)) assign(id,'')
 else if(lane==='active') emit('drop',id)
}
function pick(ticket:Ticket,event:MouseEvent) {
 const board=(event.currentTarget as HTMLElement).closest('.lanes')!
 const order=Array.from(board.querySelectorAll<HTMLElement>('[data-ticket]')).map(card=>card.dataset.ticket!)
 emit('select',ticket,event,order)
}
function assign(id:string,groupId:string) {over.value=false;emit('assignGroup',id,groupId)}
</script>
<template>
 <div class="lanes" aria-label="Ticket board">
  <section v-for="lane in lanes" :key="lane.id" :data-lane="lane.id" :aria-label="lane.title" class="lane" :class="{dropzone:lane.id==='active' && over}" @dragover.prevent="over=lane.id==='active'" @dragleave="over=false" @drop.prevent="drop($event,lane.id)">
    <header class="lane-header"><div><span class="lane-icon" :class="lane.id"><component :is="lane.icon" :size="15"/></span><h2>{{ lane.title }}</h2><span class="count">{{ tickets.filter(t=>matchesStatus(t.status,lane.id)).length }}</span></div><button v-if="lane.id==='todo'" class="icon-button" aria-label="Create ticket in Todo" @click="emit('create')"><Plus :size="17"/></button></header>
    <p class="lane-subtitle">{{ lane.subtitle }}</p>
    <div class="lane-cards">
      <button v-if="lane.id==='todo'" class="add-ticket add-group" aria-label="Add group" @click="emit('createGroup')"><Plus :size="16"/>Add group</button>
      <TicketGroup v-for="group in laneGroups(lane.id)" :key="group.id" :group="group" :lane="lane.title" :tickets="laneTickets(lane.id).filter(t=>t.group_id===group.id)" :selected-id="selectedId" :selected-ids="selectedIds" :allow-create="lane.id==='todo'" @select="pick" @create="emit('create',$event)" @edit="emit('editGroup',$event)" @remove="emit('deleteGroup',$event)" @run="emit('runGroup',$event)" @assign="assign"/>
      <TicketCard v-for="ticket in laneTickets(lane.id).filter(t=>!t.group_id)" :key="ticket.id" :ticket="ticket" :selected-id="selectedId" :selected-ids="selectedIds" @select="pick"/>
      <button v-if="lane.id==='todo'" class="add-ticket" @click="emit('create')"><Plus :size="16"/> Add ticket</button>
      <div v-if="!loading && !tickets.some(t=>matchesStatus(t.status,lane.id)) && lane.id!=='todo'" class="empty-lane"><component :is="lane.icon" :size="25"/><p>{{lane.id==='active'?'Space for your next idea':'Good things take a little work'}}</p><span>{{lane.id==='active'?'Drag a ticket here to start a run.':'Completed tickets will appear here.'}}</span></div>
    </div>
  </section>
 </div>
</template>
