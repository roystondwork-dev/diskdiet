---
description: "Remove old Playwright browsers and Cypress binaries that pile up after upgrades, keeping the newest so tests keep running."
---

# Playwright and Cypress downloads

Every Playwright or Cypress upgrade downloads new browsers or app binaries
(300 MB–1 GB each) and leaves the old ones behind.

```sh
npx diskdiet clean --only playwright,cypress
```

- **Playwright** (`ms-playwright` in your cache folder): older revisions of each browser
  (Chromium, Firefox, WebKit…) are removed; **the newest of each always stays**.
- **Cypress**: older Cypress versions are removed; the newest stays.

If a project needs an older version again, `npx playwright install` or `npx cypress
install` downloads it.
