<script setup lang="ts">
import { computed } from 'vue'
import { ChevronDown, Check } from 'lucide-vue-next'
import { DropdownMenuRoot, DropdownMenuTrigger, DropdownMenuPortal, DropdownMenuContent, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuItemIndicator } from 'reka-ui'
const props=defineProps<{modelValue:string}>()
const emit=defineEmits<{'update:modelValue':[string]}>()
const options=[{id:'all',label:'All statuses'},{id:'todo',label:'Todo'},{id:'active',label:'Doing'},{id:'done',label:'Done'}]
const label=computed(()=>options.find(option=>option.id===props.modelValue)?.label ?? 'All statuses')
</script>
<template>
 <DropdownMenuRoot><DropdownMenuTrigger class="filter status-filter" aria-label="Filter status">{{label}}<ChevronDown :size="13" aria-hidden="true"/></DropdownMenuTrigger><DropdownMenuPortal><DropdownMenuContent class="status-menu menu-surface" align="end" :side-offset="6"><DropdownMenuRadioGroup :model-value="modelValue" @update:model-value="emit('update:modelValue',String($event))"><DropdownMenuRadioItem v-for="option in options" :key="option.id" :value="option.id" :data-status="option.id" class="menu-option">{{option.label}}<DropdownMenuItemIndicator><Check :size="13" aria-hidden="true"/></DropdownMenuItemIndicator></DropdownMenuRadioItem></DropdownMenuRadioGroup></DropdownMenuContent></DropdownMenuPortal></DropdownMenuRoot>
</template>
