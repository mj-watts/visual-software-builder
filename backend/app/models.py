import base64
from typing import Literal
from pydantic import BaseModel, Field, field_validator
Provider = Literal['demo', 'codex', 'claude']
Status = Literal['todo', 'queued', 'running', 'done', 'failed', 'cancelled']

class Draft(BaseModel):
    title: str = Field(min_length=1, max_length=160)
    prompt: str = Field(min_length=1, max_length=20000)
    provider: Provider = 'demo'
    images: list[str] = Field(default_factory=list, max_length=6)
    workspace_id: str = ''
    permission: Literal['read-only', 'workspace-write'] = 'read-only'
    allow_tests: bool = False
    group_id: str = Field(default='', max_length=80)

    @field_validator('title', 'prompt')
    @classmethod
    def nonblank(cls, value):
        if not value.strip():
            raise ValueError('Must not be blank')
        return value.strip()

    @field_validator('images')
    @classmethod
    def valid_images(cls, images):
        for image in images:
            validate_image(image)
        return images

def validate_image(image):
    header, _, encoded = image.partition(',')
    if header not in {'data:image/png;base64', 'data:image/jpeg;base64', 'data:image/webp;base64'}:
        raise ValueError('Use PNG, JPEG or WebP images')
    if len(encoded) > 6_666_668:
        raise ValueError('Image exceeds 5 MB')
    decoded = base64.b64decode(encoded, validate=True)
    if not decoded or len(decoded) > 5_000_000:
        raise ValueError('Image is empty or exceeds 5 MB')

class Changes(BaseModel):
    title: str | None = Field(default=None, min_length=1, max_length=160)
    prompt: str | None = Field(default=None, min_length=1, max_length=20000)
    provider: Provider | None = None
    images: list[str] | None = Field(default=None, max_length=6)

    workspace_id: str | None = None
    permission: Literal['read-only', 'workspace-write'] | None = None
    allow_tests: bool | None = None
    group_id: str | None = Field(default=None, max_length=80)

class ChangedFile(BaseModel):
    path: str
    additions: int
    deletions: int
    diff: str

class Ticket(Draft):
    id: str
    project_id: str = 'default'
    status: Status = 'todo'
    response: str = ''
    files: list[ChangedFile] = Field(default_factory=list)
    created_at: str
    queued_at: str = ''
    latest_run_id: str = ''

Permission = Literal['read-only', 'workspace-write']
TestPreset = Literal['none', 'vitest', 'pytest']

class WorkspaceInput(BaseModel):
    path: str = Field(min_length=1, max_length=4096)
    name: str = Field(min_length=1, max_length=120)
    test_preset: TestPreset = 'none'
    test_directory: str = Field(default='.', max_length=500)

class Workspace(WorkspaceInput):
    id: str

class Run(BaseModel):
    id: str
    ticket_id: str
    snapshot: Draft
    status: Status = 'queued'
    group_run_id: str = ''
    previous_run_id: str = ''
    base_commit: str = ''
    workspace_path: str = ''
    worktree: str = ''
    test_preset: TestPreset = 'none'
    test_directory: str = '.'
    created_at: str
    started_at: str = ''
    finished_at: str = ''
    response: str = ''
    files: list[ChangedFile] = Field(default_factory=list)
    error: str = ''
    tests: str = 'not-requested'

class RunEvent(BaseModel):
    id: int
    run_id: str
    ticket_id: str
    type: str
    message: str
    created_at: str


class GroupInput(BaseModel):
    name: str = Field(min_length=1, max_length=80)
    color: str = Field(default='orange', pattern=r'^(orange|blue|green|amber|plum|#[0-9a-fA-F]{6})$')

    @field_validator('name')
    @classmethod
    def nonblank(cls, value):
        if not value.strip():
            raise ValueError('Group name must not be blank')
        return value.strip()

class Group(GroupInput):
    id: str
    project_id: str = 'default'

class GroupAssignment(BaseModel):
    group_id: str = Field(default='', max_length=80)


class Project(BaseModel):
    name: str = Field(default='My first project', min_length=1, max_length=120)
    description: str = Field(default='', max_length=2000)
    screenshot: str = ''

    @field_validator('screenshot')
    @classmethod
    def valid_screenshot(cls, value):
        if value:
            validate_image(value)
        return value

    @field_validator('name')
    @classmethod
    def nonblank(cls, value):
        if not value.strip():
            raise ValueError('Project title must not be blank')
        return value.strip()

    @field_validator('description')
    @classmethod
    def trimmed(cls, value):
        return value.strip()

class ManagedProject(Project):
    id: str
    deleted: bool = False
