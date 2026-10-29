import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// Backend the dev server proxies to; override with AIGW_DEV_BACKEND for remote stacks.
const backend = process.env.AIGW_DEV_BACKEND || 'http://localhost:8080';
const proxy = {
  '/api/admin': { target: backend, changeOrigin: true },
  '/v1': { target: backend, changeOrigin: true }
};

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  server: { port: 5173, proxy }
});
