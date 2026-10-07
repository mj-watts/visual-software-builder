<script setup lang="ts">
import { ChevronDown, Check } from 'lucide-vue-next'
import { DropdownMenuRoot, DropdownMenuTrigger, DropdownMenuPortal, DropdownMenuContent, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuItemIndicator, DropdownMenuItem, DropdownMenuSeparator } from 'reka-ui'
import type { ManagedProject } from '../types'
defineProps<{projects:ManagedProject[];selectedId:string;label:string;busy:boolean}>()
defineEmits<{select:[string];manage:[]}>()
</script>
<template>
 <DropdownMenuRoot><DropdownMenuTrigger class="project-switch" aria-label="Switch project" :disabled="busy"><span>{{label}}</span><ChevronDown :size="13" aria-hidden="true"/></DropdownMenuTrigger><DropdownMenuPortal><DropdownMenuContent class="project-switch-menu menu-surface" align="start" :side-offset="8" aria-label="Projects">
  <DropdownMenuRadioGroup :model-value="selectedId"><DropdownMenuRadioItem v-for="project in projects.filter(p=>!p.deleted)" :key="project.id" :value="project.id" :data-project-option="project.id" class="menu-option" @select="$emit('select',project.id)"><span>{{project.name}}</span><span v-if="project.id===selectedId" class="current-project-label">Current</span><DropdownMenuItemIndicator><Check :size="14" aria-hidden="true"/></DropdownMenuItemIndicator></DropdownMenuRadioItem></DropdownMenuRadioGroup>
  <p v-if="!projects.some(p=>!p.deleted)" class="small-note">No active projects.</p><DropdownMenuSeparator class="project-menu-divider"/><DropdownMenuItem class="menu-option" @select="$emit('manage')">Manage projects</DropdownMenuItem>
 </DropdownMenuContent></DropdownMenuPortal></DropdownMenuRoot>
</template>
