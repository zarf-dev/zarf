import adapter from "@sveltejs/adapter-static";
import preprocess from 'svelte-preprocess';

const config = {
  // Consult https://github.com/sveltejs/svelte-preprocess
  // for more information about preprocessors
  preprocess: preprocess(),
  kit: {

    files: {
      assets: 'src/ui/static',
      lib: 'src/ui/lib',
      params: 'src/ui/params',
      routes: 'src/ui/routes',
      serviceWorker: 'src/ui/service-worker',
      appTemplate: 'src/ui/app.html'
    },
    adapter: adapter({
      pages: 'build/ui',
      assets: 'build/ui',
      fallback: "index.html",
    }),
  },
};

export default config;