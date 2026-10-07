import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
export default defineConfig({plugins:[vue()],server:{proxy:{'/api':process.env.SWIMLANE_API_URL ?? 'http://127.0.0.1:8080'}},test:{environment:'jsdom',include:['tests/**/*.test.ts'],coverage:{provider:'v8',include:['src/**/*.ts','src/**/*.vue'],reporter:['text','json','json-summary'],thresholds:{lines:80,functions:80,branches:80,statements:80}}}})
