# 24. Migrate documentation to Starlight

Date: 2024-03-30

## Status

Accepted

## Context

The current documentation site is built with Docusaurus 2. While Docusaurus has served us well, it has some limitations, primarily around performance and dependency management.

### Proposed Solutions

1. [**Starlight**](https://starlight.astro.build/) is a new documentation site generator that is built with performance in mind. It is a lightweight, fast, and flexible static site generator powered by [Astro](https://astro.build) that is designed to be easy to use and extend.

    - **Pros**:
        - Fast performance.
        - Easy to extend with custom components in any framework.
        - Built-in support for MDX.
        - Simplified build process.
        - Better control over dependencies.
        - Client-side static site search built-in.
        - Excellent default theme.
    - **Cons**:
        - Newer project with a smaller ecosystem compared to others.
        - Much fewer plugins and themes available compared to other static site generators.
        - Only absolute URLs are supported for cross-site links (relative image URLs are supported).

2. [**Hugo**](https://gohugo.io/): is another popular static site generator that is known for its speed and flexibility. It is written in Go and has a large ecosystem of themes and plugins.

    - **Pros**:
        - Fast performance.
        - Large ecosystem of themes and plugins.
        - By default no JavaScript is required.
    - **Cons**:
        - Fewer built-in features compared to others, much more of a DIY approach.
        - Steeper learning curve compared to others.
        - More complex build process.
        - Not as easy to extend with custom components.
        - Theme management is abysmal w/ a combination of Go modules, Git submodules, and NPM packages.
        - Zero documentation themes the team liked, leading to the quandry of whether to build, and having to maintain, a custom theme.

3. [**Material for MkDocs**](https://squidfunk.github.io/mkdocs-material/): is a popular static site generator geared towards project documentation. The Material for MkDocs theme provides a clean and responsive design.

    - **Pros**:
        - _Amazing_ default theme and developer experience for writing docs.
        - Easy to use and extend.
        - Large ecosystem of plugins.
    - **Cons**:
        - Not as flexible as Hugo or Starlight.
        - Making custom components can be more challenging.
        - Brings in Python dependencies (not a really huge detractor, but the team would like to reduce dependencies on different languages where possible).

## Decision

To simplify the development process and improve the performance of the documentation site, we have decided to migrate to Starlight. This is based upon a number of factors:

- Small dependency footprint.
- Fast performance.
- Easy to extend with custom components.
- Built-in support for MDX.
- The majority of the features we need are built-in and require minimal configuration.
  - Client-side static site search
  - Sane spacing and typography
  - Dark mode + light mode
  - Syntax highlighting
  - Responsive design + mobile-friendly
  - Tabs + Admonitions

## Consequences

- During the transition, we will need to update the existing documentation content to work with the new site generator. Additionally, the site archictecture will be re-evaluated and optimized. This _will_ result in many links to current content breaking.
- Every documentation generator has its quirks and limitations, so we will need to adapt to the new workflow and learn how to best utilize the features of Starlight.
- The migration will take time and effort, but the benefits of improved performance and flexibility will be worth it in the long run.
- The team will need to learn how to use Astro and Starlight, which may require some training and experimentation.
