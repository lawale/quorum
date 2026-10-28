import { defineConfig } from 'vite';

export default defineConfig({
  build: {
    lib: {
      entry: 'src/index.ts',
      formats: ['iife', 'es'],
      name: 'QuorumEmbed',
      fileName: (format) => (format === 'iife' ? 'embed.js' : 'index.mjs'),
    },
    outDir: 'dist',
    emptyOutDir: true,
  },
});
