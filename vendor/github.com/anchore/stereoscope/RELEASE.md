# Release process

A release of stereoscope results in:
- a new semver git tag from the current tip of the main branch
- a new [github release](https://github.com/anchore/stereoscope/releases) with a changelog

A new release can be created by running:
```
make release
```

When prompted to continue (`Do you want to trigger a release for version?`) review the generated changelog. If it is inaccurate then select `n` to cancel. Then you can edit issue/PR titles and labels, restarting the release process again.

Follow the subsequent run of the [github action workflow](https://github.com/anchore/stereoscope/actions/workflows/release.yaml) to see the progress / result of the release.
