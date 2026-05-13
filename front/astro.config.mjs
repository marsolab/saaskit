// @ts-check
import { defineConfig } from 'astro/config';

import sitemap from '@astrojs/sitemap';

import partytown from '@astrojs/partytown';

import mdx from '@astrojs/mdx';

import react from '@astrojs/react';

import cloudflare from '@astrojs/cloudflare';

import tailwindcss from '@tailwindcss/vite';

import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
  integrations: [sitemap(), partytown(), starlight({ title: 'Docs' }), mdx(), react()],
  adapter: cloudflare(),
  vite: {
    plugins: [tailwindcss()]
  }
});
