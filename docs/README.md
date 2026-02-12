# Coderaft Docs

This site is built with [Astro + Starlight](https://starlight.astro.build).

### Setup

```bash
# from this folder
pnpm install
```

### Local development

```bash
pnpm dev
```

### Build

```bash
pnpm build
```

### Preview a build

```bash
pnpm preview
```

If you see an error about Corepack/pnpm, enable Corepack or install pnpm:

```bash
# Option A: Enable Corepack (recommended)
# Run bash as Administrator if needed
corepack enable; corepack prepare pnpm@latest --activate

# Option B: Install pnpm
npm install -g pnpm
```

## Contributing to the docs

We love documentation improvements—typos, clarity tweaks, new guides, and examples are all welcome. This site uses Astro + Starlight and lives entirely in this `docs/` folder.

### Docs structure

- Content pages: `src/content/docs/`
	- `index.mdx` is the docs homepage
	- Subfolders group topics; files inside become sidebar items automatically
- Static assets (images, downloads): `public/`
	- Example: place an image at `public/images/example.png` and reference it as `/images/example.png`
- Theme and components: `src/` (e.g., `src/components/`)

### Add or edit a page

1. Create or edit a Markdown/MDX file under `src/content/docs/...`
2. Add minimal frontmatter at the top:

```md
---
title: My New Guide
description: Brief one‑liner that appears in search and previews.
# Optional Starlight fields:
# sidebar:
#   label: Short label for the sidebar (defaults to title)
# slug: my-new-guide     # sets the URL path segment
---

# Page content starts here
```

3. Start the local server to preview: `pnpm dev`

#### Notes:
- Prefer short paragraphs and descriptive headings (H2/H3) so the auto ToC works well.
- Use absolute links for site assets from `public/` (e.g., `/images/...`).
- For MDX, you can import and use components when needed.

### Style and conventions

- Keep titles concise; first H1 should match the page’s purpose.
- Use present tense and active voice; prefer simple, clear language.
- Show commands first, then output. Mark commands with the correct language (bash, powershell, json, yaml, etc.).
- Use inline code for flags and filenames (e.g., `--help`, `coderaft.yaml`).
- Include prerequisites at the top of task/guide pages.
- Cross‑link related pages with relative links when possible.

### Run checks locally

- Preview locally: `pnpm dev`
- Build to validate: `pnpm build` then `pnpm preview`

### Submitting your changes

- For general contribution guidelines (branching, commit format, PR checklist), see the repository’s `CONTRIBUTING.md`.
- In your PR description, mention “Docs:” and link the pages you changed or added.
- Screenshots are helpful for visual changes.

That’s it—open a PR and we’ll review it. Thanks for improving the docs!
