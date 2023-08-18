# Zarf Hugo Doc Site

This site is built using the [Defense Unicorns](https://github.com/defenseunicorns/defense-unicorns-hugo-theme) theme
for Hugo

## Contributing content

Content is created using standard markdown with [frontmatter](https://main--starlit-valkyrie-7f1dd9.netlify.app/docs/adding-content/content/#page-frontmatter) to control page order, title, menu title, etc. More rich
user interactions can be implemented using the [shortcodes from the theme](https://main--starlit-valkyrie-7f1dd9.netlify.app/docs/adding-content/shortcodes/) or by creating local shortcodes for
specialized interactions.

## Converting documents

The current documentation is formatted and structured for Docusaurus. In addition, the auto-generated docs are built
with that format. The `setup-docs.sh` script is run by `npm start` to copy some example files to reachable locations, convert all of the existing
documentation, and move the Docusaurus documentation to a backup folder pending deletion. Once the conversion
meets expectations, the old documentation will be removed and cleanup will begin.

### Post Conversion TODOs

- Modify the API documentation generator to create documents with the correct frontmatter and shortcodes
- Consolidate all scripts used for generating automated docs to simplify maintenance.

## Running Locally

To run the site for local development:

```bash
npm start
```

Then navigate to [http://localhost:1313/](http://localhost:1313/)
