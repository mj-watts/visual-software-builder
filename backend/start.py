"""Load server-local .env without executing shell code, then start localhost FastAPI."""
import os
from pathlib import Path
import uvicorn


from app.settings import load_env

if __name__ == '__main__':
    os.chdir(Path(__file__).resolve().parent)
    load_env(Path('.env'))
    uvicorn.run('app.main:app', host='127.0.0.1', port=8000, timeout_graceful_shutdown=5)
