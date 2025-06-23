# Zarf Release Process

This document provides guidance on how to create Zarf releases, address release issues, and other helpful tips.

This project uses [goreleaser](https://github.com/goreleaser/goreleaser-action) for releasing. Releases are defined in the [`.github/workflows/release.yml`](.github/workflows/release.yml) file and are triggered by pushing a signed tag starting with `v`.

## Release Cadence

The zarf release cadence aims to happen every 2 weeks at a minimum. Final scheduling is determined by the team once any release goals are met, ideally keeping to about a 1–2 week timeline between releases.

## Release Candidates

When there have been changes that could potentially impact the release process — such as updates to goreleaser, changes to GitHub runners, major testing updates, significant shifts in Zarf artifact size, or new artifacts being released — it is recommended to cut a Release Candidate (RC) first. To evaluate the criticality of a Release Candidate - perform the following:
- Review all changed files since the last tag
  - In the command-line: `git diff -stat <prev-tag>..origin/main` and then review specific files with `git diff <prev-tag>..origin/main -- path/to/file`
  - In the browser: `https://github.com/zarf-dev/zarf/compare/<prev-tag>...main`
- Check for changes to known areas of CI/Release failure
  - `goreleaser` updates
  - Testing structure refactor
  - Known brittle or flaky tests
- Review any changes to the GitHub Runners
  - [runner images](https://github.com/actions/runner-images)

Tag the candidate with a suffix of the form `-rcX` and push it:

```bash
# Example RC tag
git tag -sa v0.50.0-rc1 -m "v0.50.0-rc1"
git push origin v0.50.0-rc1
```

The `.goreleaser.yaml` configuration will automatically mark `-rc` tags as prereleases in GitHub:

```yaml
release:
  prerelease: auto
```

Once the prerelease artifacts are published, a Homebrew Tap PR is created. It is at the team's discretion whether to merge or close this PR to prevent publishing to brew users until the final release is ready.

## Release Checklist

* [ ] Review open [Pull Requests](https://github.com/defenseunicorns/zarf/pulls)
* [ ] Cut the new release by tagging and pushing:

  ```bash
  git tag -sa vX.Y.Z -m "vX.Y.Z"
  git push origin vX.Y.Z
  ```
* [ ] Review the GitHub release:
  * [ ] Add a summary of release updates and any required documentation around updates or breaking changes
* [ ] Ensure goreleaser workflows execute successfully and review the release assets
* [ ] Review, approve, and merge the [homebrew-tap](https://github.com/defenseunicorns/homebrew-tap) PR for the zarf release

## Release Issues

### A release is "broken" and should not be used

Rather than removing a broken release, mark it in the release notes and cut a new release that fixes the issue(s).

* **Manual approach:**

  1. Find the impacted release on GitHub
  2. Edit the release notes and add this warning at the top:

     ```md
     >[!WARNING]
     >PLEASE USE A NEWER VERSION (there are known issues with this release)
     ```
  3. Create and publish a new release to address the issues
