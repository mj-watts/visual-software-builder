"""Supervised argv-only child processes with bounded output and process-group cleanup."""
import asyncio
import os
import signal

MAX_OUTPUT = 5_000_000

async def stop(process):
    if process.returncode is not None:
        return
    signal_group(process.pid, signal.SIGTERM)
    try:
        await asyncio.wait_for(process.wait(), 2)
    except TimeoutError:
        signal_group(process.pid, signal.SIGKILL)
        await process.wait()


def signal_group(pid, signal_type):
    try:
        os.killpg(pid, signal_type)
    except ProcessLookupError:
        pass


async def pump(stream, kind, emit, budget):
    while line := await stream.readline():
        budget[0] += len(line)
        if budget[0] > MAX_OUTPUT:
            raise RuntimeError('Agent output exceeded the 5 MB limit.')
        await emit(kind, line.decode('utf8', errors='replace').rstrip())


async def feed(process, text):
    try:
        process.stdin.write(text.encode())
        await process.stdin.drain()
    except (BrokenPipeError, ConnectionResetError):
        pass
    finally:
        process.stdin.close()


async def run_process(argv, cwd, text, emit, timeout, env):
    process = await asyncio.create_subprocess_exec(*argv, cwd=str(cwd), env=env,
        stdin=asyncio.subprocess.PIPE, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE,
        start_new_session=True, limit=1_000_000)
    budget = [0]
    tasks = [asyncio.create_task(pump(process.stdout, 'stdout', emit, budget)),
             asyncio.create_task(pump(process.stderr, 'stderr', emit, budget)),
             asyncio.create_task(feed(process, text))]
    try:
        async with asyncio.timeout(timeout):
            await asyncio.gather(*tasks)
            code = await process.wait()
        if code:
            raise RuntimeError(f'Agent process exited with code {code}. See run activity for details.')
    finally:
        # Terminate surviving descendants too, including after the parent exits.
        await stop(process)
        signal_group(process.pid, signal.SIGKILL)
        for task in tasks:
            task.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)
