"""Durable dependencies keep group prompts ordered and stop on failed test gates."""
from uuid import uuid4
from fastapi import HTTPException
from .runs import build_run, mark_queued


def group_tickets(store, group_id):
    members = [ticket for ticket in store.all() if ticket.group_id == group_id]
    if any(ticket.status in ['queued', 'running'] for ticket in members):
        raise HTTPException(409, 'This group already has queued or running tickets.')
    tickets = [ticket for ticket in members if ticket.status == 'todo']
    if not tickets:
        raise HTTPException(409, 'This group has no Todo tickets to run.')
    return tickets


def checked_run(store, ticket, settings):
    run = build_run(store, ticket, settings)
    if ticket.provider != 'demo':
        require_tests(run)
    return run


def require_tests(run):
    if not run.snapshot.allow_tests or run.test_preset == 'none':
        raise HTTPException(409, 'Group runs require tests on every real-agent ticket. Enable Run configured tests and select a repository test preset.')


def queue_group(store, group_id, settings):
    tickets = group_tickets(store, group_id)
    runs = [checked_run(store, ticket, settings) for ticket in tickets]
    batch_id = uuid4().hex
    previous = ''
    pairs = []
    for run, ticket in zip(runs, tickets):
        run.group_run_id = batch_id
        run.previous_run_id = previous
        previous = run.id
        pairs.append((run, mark_queued(ticket, run)))
    store.enqueue_group(pairs)
    return tickets


def passed(run):
    if run.status != 'done':
        return False
    return run.snapshot.provider == 'demo' or run.tests == 'passed'


def blocked_reason(store, run):
    if not run.previous_run_id:
        return ''
    previous = store.run(run.previous_run_id)
    if previous and passed(previous):
        return ''
    return 'Group stopped: the preceding prompt did not complete with passing tests. This prompt was not executed. Retry it explicitly after resolving the earlier failure.'
