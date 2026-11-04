# Link Checks

Install [lychee 0.24.2](https://github.com/lycheeverse/lychee/releases/tag/lychee-v0.24.2), then run from the repository root:

Check all links (MDs and site)
```bash
make check-links 
```

Check only sit link:
```bash
make check-site-links
```

Check only markdown links:
```bash
make check-md-links
```

Each check builds the versioned site first. This is also required by the Markdown check to validate links into the documentation. Add `CHECK_EXTERNAL=true` to check external HTTP URLs. Reports are saved in `.lychee/results/`.

The TOML files in this directory define shared, site, Markdown, external, and authentication-check policies. `LYCHEE` can override the binary path.
