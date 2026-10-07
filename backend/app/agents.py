import asyncio
from typing import Protocol
from .models import Ticket, ChangedFile

class Agent(Protocol):
    async def run(self, ticket: Ticket) -> tuple[str, list[ChangedFile]]: ...

class DemoAgent:
    def __init__(self, delay=3):
        self.delay = delay

    async def run(self, ticket):
        await asyncio.sleep(self.delay)
        response = f'Simulated run complete.\n\nI reviewed the prompt for “{ticket.title}” and prepared an illustrative implementation and test diff.\n\nThis is a demo response. No AI provider was called and no repository files were changed. Connect a coding agent adapter to perform real work.'
        files = [ChangedFile(path='src/components/Feature.vue', additions=3, deletions=1,
            diff='--- a/src/components/Feature.vue\n+++ b/src/components/Feature.vue\n@@ -1,3 +1,5 @@\n <template>\n-  <section />\n+  <section aria-label="Feature">\n+    <h2>Ready for your next idea</h2>\n+  </section>\n </template>'),
            ChangedFile(path='src/components/Feature.test.ts', additions=3, deletions=0,
            diff="--- /dev/null\n+++ b/src/components/Feature.test.ts\n@@ -0,0 +1,3 @@\n+it('renders the feature heading', () => {\n+  expect(wrapper.text()).toContain('Ready for your next idea')\n+})")]
        return response, files
