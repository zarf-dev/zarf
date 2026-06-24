// Loads the Zarf JSON schema matching the page's docs version. build-versions.mjs
// stages each version's schema as `src/assets/schema/<slug>.json` (and the current
// checkout as `latest.json`); the slug is derived from the page URL so the shared
// schema components render the right version.

const schemas = import.meta.glob<{ default: Record<string, any> }>("../assets/schema/*.json");

const VERSION_SLUG = /^v\d+-\d+$/;

export async function loadSchema(pathname: string): Promise<Record<string, any>> {
  const segment = pathname.split("/").filter(Boolean)[0] ?? "";
  const slug = VERSION_SLUG.test(segment) ? segment : "latest";
  const loader = schemas[`../assets/schema/${slug}.json`] ?? schemas["../assets/schema/latest.json"];
  if (!loader) throw new Error(`No schema asset found for "${slug}" (is prebuild staged?)`);
  return (await loader()).default;
}
