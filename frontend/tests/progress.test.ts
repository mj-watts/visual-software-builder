import { it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import TicketCard from '../src/components/TicketCard.vue'
import type { Ticket } from '../src/types'
const ticket:Ticket={id:'SW-001',title:'Task',prompt:'Build it',provider:'demo',images:[],files:[],response:'',status:'queued',created_at:'',queued_at:'',latest_run_id:'',workspace_id:'',permission:'read-only',allow_tests:false}
it('shows waiting and indeterminate progress, then a completion tick',async()=>{
 const w=mount(TicketCard,{props:{ticket}})
 expect(w.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('0')
 expect(w.get('[role="progressbar"]').attributes('aria-valuetext')).toBe('Waiting to start')
 await w.setProps({ticket:{...ticket,status:'running'}})
 expect(w.get('[role="progressbar"]').attributes()).not.toHaveProperty('aria-valuenow')
 expect(w.get('[role="progressbar"]').attributes('aria-valuetext')).toBe('Agent is working')
 await w.setProps({ticket:{...ticket,status:'done'}})
 expect(w.find('[role="progressbar"]').exists()).toBe(false)
 expect(w.get('[aria-label="Completed"]').classes()).toContain('completion-mark');w.unmount()
})
it('never shows a success tick for failed, cancelled or unstarted tickets',async()=>{
 const w=mount(TicketCard,{props:{ticket:{...ticket,status:'todo'}}})
 for(const status of ['todo','failed','cancelled'] as const) {
  await w.setProps({ticket:{...ticket,status}})
  expect(w.find('[aria-label="Completed"]').exists()).toBe(false)
  expect(w.find('[role="progressbar"]').exists()).toBe(false)
 }
 w.unmount()
})
