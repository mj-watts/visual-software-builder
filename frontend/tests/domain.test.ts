import { describe, it, expect } from 'vitest'
import { validateDraft, visibleTickets, diffClass, imageAllowed } from '../src/domain'
describe('ticket rules',()=>{
 it('requires title and prompt',()=>{expect(validateDraft('','')).toBe('Give your ticket a title.');expect(validateDraft('Title','')).toBe('Write a prompt before running this ticket.');expect(validateDraft('Title','Prompt')).toBe('')})
 it('filters by title, id and prompt',()=>{const tickets=[{id:'SW-1',title:'Navigation',prompt:'Add links',status:'todo'}] as any;expect(visibleTickets(tickets,'links','all')).toHaveLength(1);expect(visibleTickets(tickets,'SW-1','todo')).toHaveLength(1);expect(visibleTickets(tickets,'missing','all')).toHaveLength(0);expect(visibleTickets(tickets,'','done')).toHaveLength(0)})
 it('classifies diff lines',()=>{expect(diffClass('+new')).toBe('added');expect(diffClass('-old')).toBe('removed');expect(diffClass('@@ section')).toBe('hunk');expect(diffClass(' context')).toBe('context')})
 it('limits pasted image types and size',()=>{expect(imageAllowed('image/png',100)).toBe(true);expect(imageAllowed('image/jpeg',100)).toBe(true);expect(imageAllowed('image/webp',100)).toBe(true);expect(imageAllowed('image/svg+xml',100)).toBe(false);expect(imageAllowed('image/png',6e6)).toBe(false)})
})
