# CLAUDE.md - Website Module


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

```bash
# Serve the static site and verify home page renders with the expected index
# Preferred: use the production container image.
docker inspect helixagent-website:latest >/dev/null 2>&1 && \
  docker run --rm -d -p 8080:8080 --name hw-demo helixagent-website:latest && \
  sleep 2 && curl -fsS http://localhost:8080/ | grep -qi 'HelixAgent' && \
  echo '✓ Website serves production image' && \
  docker stop hw-demo
# If no pre-built image:
[ -z "$(docker images -q helixagent-website:latest 2>/dev/null)" ] && \
  echo 'Build first: cd Website && docker build -t helixagent-website:latest .'
```
Expect: HTTP 200 with `HelixAgent` in the served HTML. For full flow testing with Playwright, add Playwright specs that drive the deployed image (the current repo does not ship Playwright tests).


## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- All validation is run manually via scripts or Makefile targets
- This rule is permanent and non-negotiable

## Overview

This directory contains all user-facing documentation for the HelixAgent project website:
user manuals, video courses, build scripts, styles, and static assets.

**Module path:** `Website/` (not an independent Go module — content only)

## Structure

```
Website/
  user-manuals/     # Step-by-step guides (01-47)
  video-courses/    # Video course content (course-*, video-course-*, courses-*/)
  scripts/          # Build and validation scripts
  styles/           # Stylesheet assets
  public/           # Static public assets
  build.sh          # Website build script
  README.md         # Module overview
  CLAUDE.md         # This file
  AGENTS.md         # Agent development guidelines
```

## Content Standards

### Markdown Formatting

- Standard CommonMark Markdown
- Headings: `#` for title, `##` for sections, `###` for subsections
- Code blocks: triple backticks with explicit language identifier (` ```bash `, ` ```go `, etc.)
- Line length: 80 chars preferred, 120 chars maximum
- Blank line between every block element (headings, paragraphs, lists, code blocks)

### User Manuals (`user-manuals/`)

- Sequential numbering: `NN-<topic-slug>.md` (e.g., `48-helix-memory-guide.md`)
- Next manual number: **48**
- Each manual must include: overview, prerequisites, step-by-step instructions,
  real curl/command examples, troubleshooting section
- Always use live API endpoints (`http://localhost:7061/v1/...`)
- No placeholder responses — show realistic output

### Video Courses (`video-courses/`)

- Primary series: `course-NN-<topic>.md` — next number: **77**
- Extended series: `video-course-NN-<topic>.md` — follows same sequence as primary
- Each course file must include: learning objectives, lesson outline, code examples,
  exercises, and a summary
- Batch subdirectories (`courses-NN-MM/`) group related courses; add new ones as needed

## Key Rules

- **NEVER delete existing content** — only add or update
- **NEVER rename existing files** — external links may depend on filenames
- **Preserve ALL existing headings and structure** when updating a file
- Add new sections at the end of existing files unless a logical insert point exists
- Keep all examples runnable against a local HelixAgent instance (`localhost:7061`)
- Cross-reference related manuals: `[Provider Configuration](../user-manuals/02-provider-configuration.md)`
- Cross-reference related courses: `[Course 03](../video-courses/course-03-deployment.md)`

## Validation

No build step required — content is pure Markdown. Manual validation steps:

1. Check all internal links resolve to existing files
2. Verify curl examples use correct port (`7061`) and path prefixes (`/v1/`)
3. Confirm sequential numbering has no gaps
4. Ensure new manuals are listed in `user-manuals/README.md`

## Resource Limits

Any scripts under `scripts/` must respect the project-wide resource limit policy:
30-40% of host resources. Use `nice -n 19` and `ionice -c 3` for any background
processing scripts.

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none (content only) |
| Downstream (these import this module) | none |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
