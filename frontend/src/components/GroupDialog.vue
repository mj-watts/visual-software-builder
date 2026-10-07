<script setup lang="ts">
import { ref, watch } from 'vue'
import type { Group } from '../types'
import { groupPalette, groupAppearance } from '../groupColors'
import PreviewDialog from './PreviewDialog.vue'
const props=defineProps<{open:boolean;group?:Group;busy:boolean;error:string}>()
const emit=defineEmits<{'update:open':[boolean];save:[string,Omit<Group,'id'>]}>()
const name=ref(''),color=ref<Group['color']>('orange')
const customColor=ref('#426b57')
watch(()=>props.open,()=>{name.value=props.group?.name ?? '';color.value=props.group?.color ?? 'orange';if(color.value.startsWith('#'))customColor.value=color.value})
function chooseCustom(value:string) {customColor.value=value;color.value=value as Group['color']}
function save() {emit('save',props.group?.id ?? '',{name:name.value.trim(),color:color.value})}
</script>
<template>
 <PreviewDialog :open="open" :title="group?'Edit group':'New group'" description="Keep related ideas together across your board." @update:open="emit('update:open',$event)">
  <form class="group-form" @submit.prevent="save"><label for="group-name">Group name</label><input id="group-name" v-model="name" required maxlength="80" placeholder="e.g. Navbar"/><fieldset class="group-colours"><legend>Group colour</legend><div class="colour-options"><button v-for="choice in groupPalette" :key="choice.name" type="button" class="colour-swatch" :style="{backgroundColor:choice.hex,color:groupAppearance(choice.name)['--group-ink']}" :aria-label="`Use ${choice.name} colour`" :title="choice.name" :aria-pressed="color===choice.name" @click="color=choice.name"><span v-if="color===choice.name" aria-hidden="true">✓</span></button><label class="custom-colour" :class="{chosen:color.startsWith('#')}"><input type="color" aria-label="Custom group colour" :value="customColor" @input="chooseCustom(($event.target as HTMLInputElement).value)"/><span>Custom</span></label></div></fieldset><p v-if="error" class="error" role="alert">{{error}}</p><button class="primary" :disabled="busy || !name.trim()" type="submit">{{group?'Save group':'Create group'}}</button></form>
 </PreviewDialog>
</template>
