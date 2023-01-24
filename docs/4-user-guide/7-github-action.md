# Getting started - Github Action

The [setup-zarf](https://github.com/defenseunicorns/setup-zarf) Github action is an officially supported action to install any version of Zarf and it's `init` package with zero added dependencies.

## Example Usage - Creating a Package

```yaml
# .github/workflows/zarf-package-create.yml
jobs:
  create_pacakge:
    runs-on: ubuntu-latest

    name: Create my cool Zarf Package
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 1

      - name: Install Zarf
        uses: defenseunicorns/setup-zarf@main # use action's main branch
        with:
          version: v0.22.2 # any valid zarf version, leave blank to use latest

      - name: Create the package
        run: zarf package create --confirm
```

More examples are located in the action's [README.md](https://github.com/defenseunicorns/setup-zarf#readme)
