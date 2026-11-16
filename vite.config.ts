import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    // three.js makes the resume viewer chunk ~540 kB, but it only loads when
    // someone opens the resume — nothing on first paint waits on it.
    chunkSizeWarningLimit: 600,
  },
})
