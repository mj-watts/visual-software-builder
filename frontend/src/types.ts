export type Status = 'todo' | 'queued' | 'running' | 'done' | 'failed' | 'cancelled'
export type Provider = 'demo' | 'codex' | 'claude'
export interface ChangedFile { path:string; additions:number; deletions:number; diff:string }
export interface Draft { title:string; prompt:string; provider:Provider; images:string[]; workspace_id:string; permission:'read-only'|'workspace-write'; allow_tests:boolean; group_id?:string }
export interface Ticket extends Draft { id:string; status:Status; response:string; files:ChangedFile[]; created_at:string; queued_at:string; latest_run_id:string }
export interface ProviderInfo { id:Provider; name:string; available:boolean; reason:string }
export interface Workspace { id:string; path:string; name:string; test_preset:'none'|'vitest'|'pytest'; test_directory:string }
export interface Run { id:string; ticket_id:string; snapshot:Draft; status:Status; group_run_id?:string; previous_run_id?:string; base_commit:string; worktree:string; created_at:string; started_at:string; finished_at:string; response:string; files:ChangedFile[]; error:string; tests:string }
export interface RunEvent { id:number; run_id:string; ticket_id:string; type:string; message:string; created_at:string }
export interface Configuration { providers:ProviderInfo[]; workspace_roots:string[]; timeout_seconds:number }

export interface Group { id:string; name:string; color:GroupColour }

export type GroupColour = 'orange'|'blue'|'green'|'amber'|'plum'|`#${string}`

export interface Project { name:string; description:string; screenshot?:string }
export interface ManagedProject extends Project { id:string; deleted:boolean }
