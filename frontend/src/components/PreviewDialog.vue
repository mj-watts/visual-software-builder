<script setup lang="ts">
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription, DialogClose } from 'reka-ui'
import { X } from 'lucide-vue-next'
defineProps<{open:boolean;title:string;description:string}>()
defineEmits<{ 'update:open':[boolean] }>()
</script>
<template>
  <DialogRoot :open="open" @update:open="$emit('update:open',$event)">
    <DialogPortal><DialogOverlay class="dialog-overlay"/><DialogContent class="dialog-content">
      <header class="dialog-header"><div><DialogTitle>{{ title }}</DialogTitle><DialogDescription>{{ description }}</DialogDescription></div><DialogClose class="icon-button" aria-label="Close preview"><X :size="20"/></DialogClose></header>
      <slot/>
    </DialogContent></DialogPortal>
  </DialogRoot>
</template>
