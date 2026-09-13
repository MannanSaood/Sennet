import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  server: { proxy: { '/api': 'http://127.0.0.1:8080', '/v1': 'http://127.0.0.1:8080', '/health': 'http://127.0.0.1:8080' } },
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: { rollupOptions: { output: { manualChunks(id) {
    if (id.includes('recharts') || id.includes('d3-')) return 'charts';
    if (id.includes('firebase')) return 'firebase';
    if (id.includes('highlight.js')) return 'highlight';
    if (id.includes('react-markdown')) return 'markdown-react';
    if (/remark-|rehype-|unified|micromark|mdast|hast|unist/.test(id)) return 'markdown-core';
    if (id.includes('node_modules/react') || id.includes('react-router') || id.includes('@tanstack/react-query')) return 'react-core';
  } } } },
})
