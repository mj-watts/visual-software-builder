import { ref } from 'vue'
type Modifiers = Pick<MouseEvent,'shiftKey'|'ctrlKey'|'metaKey'>
export function useTicketSelection() {
 const ids=ref<string[]>([]),anchor=ref('')
 function clear() {ids.value=[];anchor.value=''}
 function toggle(id:string) {ids.value=ids.value.includes(id)?ids.value.filter(value=>value!==id):[...ids.value,id];anchor.value=id}
 function range(id:string,order:string[],additive:boolean) {
  const start=order.indexOf(anchor.value),end=order.indexOf(id)
  const values=start<0?[id]:order.slice(Math.min(start,end),Math.max(start,end)+1)
  ids.value=additive?[...new Set([...ids.value,...values])]:values
  if(start<0)anchor.value=id
 }
 function click(id:string,event:Modifiers,order:string[]) {
  if(event.shiftKey){range(id,order,event.ctrlKey || event.metaKey);return false}
  if(event.ctrlKey || event.metaKey){toggle(id);return false}
  ids.value=[];anchor.value=id;return true
 }
 function retain(available:string[]) {ids.value=ids.value.filter(id=>available.includes(id))}
 return {ids,clear,click,retain}
}
