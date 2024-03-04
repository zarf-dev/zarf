# 12. Local Image Support Via Docker

Date: 2023-02-06

## Status

Accepted

## Context

There has been a long-standing usability gap with Zarf when doing local development due to a lack of local image support. A solution was merged in, [#1173](https://github.com/defenseunicorns/zarf/pull/1173), and released in [v0.23.4](https://github.com/defenseunicorns/zarf/releases/tag/v0.23.4) to support this feature. Unfortunately, we didn't realize there is a [glaring issue](https://github.com/defenseunicorns/zarf/issues/1214) with the implementation that causes Zarf to crash when trying to load large images into the local docker daemon.  The docker daemon support in Crane is somewhat naive and can send a machine into an OOM condition due to how the tar stream is loaded into memory from the docker save action. Crane does have an option to avoid this issue, but at the cost of being much slower to load images from docker.

We did extensive investigation into various strategies of loading docker images from the daemon including: crane, skopeo, the docker go client and executing the docker cli directly with varying levels of success. Unfortunately, some of the methods that did work well were up to 3 times slower than the current implementation, though they avoided the OOM issue. Lastly, the docker daemon save operations directly still ended up being slower than crane and docker produced a legacy format that would cause issues with [future package schema changes](https://github.com/defenseunicorns/zarf/issues/1319) we are planning for oci imports.

|                                                    | **Docker** | **Crane** |
| -------------------------------------------------- | ---------- | --------- |
| Big Bang Core (cached)                             | 3m 1s      | 1m 58s    |
| Big Bang Core (cached + skip-sbom)                 | 1m 51s     | 56s       |
| 20 GB Single-Layer Image (local registry)          |            | 6m 14s    |
| 20 GB Single-Layer Image (local registry + cached) | 5m 2s      | 2m 10s    |

## Decision

We had hoped to leverage docker caching and avoid crane caching moreso, but realized that caching was still occurring via Syft for SBOM. Additionally, the extremely-large, local-only image is actually the edge case here and we created a recommended workaround in the FAQs as well as an inline alert when a large docker image is detected. This restores behavior to what it was before the docker daemon support was added, but with the added benefit of being able to load images from the docker daemon when they are available locally.

## Consequences

For most cases this will be a seamless transition back to the previous behavior while still supporting local-only images. While this will work for large images too, it will be slow and this is automatically communicated to the user via a warning/recommendation to use a local registry.
