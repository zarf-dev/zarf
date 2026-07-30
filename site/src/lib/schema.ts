// Loads a Zarf JSON schema for the shared schema components. Two independent axes
// select the asset, both staged under `src/assets/schema/<docsSlug>/<apiVersion>.json`
// by prebuild (the current checkout, as `latest/`) and build-versions.mjs (each
// archived release, as `<slug>/`):
//   - Docs version: the page URL's leading `vN-N` slug; any other path is `latest`.
//   - API version: the `apiVersion` prop; omitted, it defaults to v1alpha1.

const schemas = import.meta.glob<{ default: Record<string, any> }>("../assets/schema/**/*.json");

const VERSION_SLUG = /^v\d+-\d+$/;
const DEFAULT_API = "v1alpha1";

export async function loadSchema(
  pathname: string,
  apiVersion: string = DEFAULT_API,
): Promise<Record<string, any>> {
  const segment = pathname.split("/").filter(Boolean)[0] ?? "";
  const docsSlug = VERSION_SLUG.test(segment) ? segment : "latest";
  const loader =
    schemas[`../assets/schema/${docsSlug}/${apiVersion}.json`] ??
    schemas[`../assets/schema/latest/${DEFAULT_API}.json`];
  if (!loader) {
    throw new Error(`No schema asset for "${docsSlug}/${apiVersion}" (is prebuild staged?)`);
  }
  return (await loader()).default;
}
