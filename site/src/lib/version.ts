/** Matches an archived-version slug segment, e.g. "v0-76". */
export const VERSION_SLUG = /^v\d+-\d+$/;

/** Filter value used for the current (unversioned) docs checkout. */
export const LATEST = "current";

/** Resolve the docs version from a URL path, falling back to the current checkout. */
export function versionFromPath(pathname: string): string {
  return pathname.split("/").find((s) => VERSION_SLUG.test(s)) ?? LATEST;
}
