import adapter from "@sveltejs/adapter-static";
import preprocess from 'svelte-preprocess';

const config = {
  // TODO: (@JMCCOY) blah blah blah
  onwarn: (warning, handler) => {
    // disable css-unused warnings
    if (warning.code.startsWith("css-unused-")) return;
    handler(warning);
  },
  root: ".",
  // Consult https://github.com/sveltejs/svelte-preprocess
  // for more information about preprocessors
  preprocess: preprocess(),
  kit: {
    files: {
      assets: 'static',
      lib: 'lib',
      params: 'params',
      routes: 'routes',
      serviceWorker: 'service-worker',
      appTemplate: 'app.html'
    },
    adapter: adapter({
      pages: '../../build/ui',
      assets: '../../build/ui',
      fallback: "index.html",
    }),
  },
};

export default config;
