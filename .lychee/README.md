# Link Checks

Install [lychee](https://github.com/lycheeverse/lychee#installation), then run from the repository root:

Test all links (Markdown and site):

```bash
make test-links
```

Test only site links:

```bash
make test-site-links
```

Test only Markdown links:

```bash
make test-md-links
```

Each check builds the versioned site first. This is also required by the Markdown check to validate links into the documentation. Add `TEST_EXTERNAL_URLS=true` to check external HTTP URLs. Reports are saved in `.lychee/results/`.

The TOML files in this directory define shared, site, Markdown, external, and authentication-check policies. `LYCHEE` can override the binary path.
