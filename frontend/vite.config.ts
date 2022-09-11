import { sveltekit } from '@sveltejs/kit/vite';
import type { UserConfig } from 'vite';

const config: UserConfig = {
  plugins: [sveltekit()],
  server: {
    fs: {
      strict: false,
    }
  },
  resolve: {
    alias: {
      "@images": __dirname + "/src/assets/images",
      "@ui": __dirname + "/node_modules/@defense-unicorns/unicorn-ui",
    }
  }
};

export default config;
