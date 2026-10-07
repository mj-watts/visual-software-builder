<script setup lang="ts">
import { ref, watch } from 'vue'
import type { Project } from '../types'
import PreviewDialog from './PreviewDialog.vue'
import { imageAllowed } from '../domain'
const props=defineProps<{open:boolean;project:Project;busy:boolean;error:string;creating?:boolean}>()
const emit=defineEmits<{'update:open':[boolean];save:[Project]}>()
const name=ref(''),description=ref('')
const screenshot=ref<string>(),imageError=ref(''),reading=ref(false),preview=ref(false)
watch(()=>props.open,()=>{name.value=props.project.name;description.value=props.project.description;screenshot.value=props.project.screenshot;imageError.value=''},{immediate:true})
function save() {emit('save',{name:name.value.trim(),description:description.value.trim(),...(screenshot.value!==undefined?{screenshot:screenshot.value}:{})})}
async function image(file?:File) {
 if(!file)return
 imageError.value=''
 if(!imageAllowed(file.type,file.size)){imageError.value='Use PNG, JPEG or WebP up to 5 MB.';return}
 reading.value=true
 try {screenshot.value=await new Promise<string>((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(String(reader.result));reader.onerror=()=>reject(new Error('Could not read screenshot.'));reader.readAsDataURL(file)})}
 catch {imageError.value='Could not read screenshot.'}
 finally {reading.value=false}
}
function upload(event:Event) {image((event.target as HTMLInputElement).files?.[0])}
function paste(event:ClipboardEvent) {const file=Array.from(event.clipboardData?.files ?? []).find(f=>f.type.startsWith('image/'));if(file){event.preventDefault();image(file)}}
</script>
<template>
 <PreviewDialog :open="open" :title="creating?'New project':'Edit project'" description="Give your project a name and describe what you want to build." @update:open="emit('update:open',$event)">
  <form class="project-form" @submit.prevent="save" @paste="paste"><label for="project-name">Project title</label><input id="project-name" v-model="name" required maxlength="120"/><label for="project-description">Project description</label><textarea id="project-description" v-model="description" maxlength="2000" placeholder="What are you building?"/>
   <label for="project-screenshot-file">Project screenshot</label><div class="project-screenshot" tabindex="0" role="group" aria-label="Project screenshot"><template v-if="screenshot"><button type="button" aria-label="Enlarge project screenshot" @click="preview=true"><img :src="screenshot" alt="Project screenshot preview"/></button><button type="button" class="secondary" @click="screenshot=''">Remove screenshot</button></template><p v-else>Upload or paste a screenshot to give your project a preview.</p><input id="project-screenshot-file" type="file" accept="image/png,image/jpeg,image/webp" :disabled="reading" @change="upload"/><span class="small-note">PNG, JPEG or WebP · up to 5 MB</span></div>
   <p v-if="imageError" role="alert" class="error">{{imageError}}</p><p v-if="error" role="alert" class="error">{{error}}</p><button class="primary" type="submit" :disabled="busy || reading || !name.trim()">{{creating?'Create project':'Save project'}}</button></form>
 </PreviewDialog>
 <PreviewDialog v-model:open="preview" title="Project screenshot" description="Preview your project image."><img :src="screenshot" class="preview-image" alt="Project screenshot"/></PreviewDialog>
</template>
