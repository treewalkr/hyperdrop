# Issue tracker: GitHub

Issues and PRDs for this repo live as GitHub issues in `treewalkr/hyperdrop`. Use the `gh` CLI for all operations.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a heredoc for multi-line bodies.
- **Read an issue**: `gh issue view <number> --comments`
- **List issues**: `gh issue list --state open --json number,title,labels --jq '.[] | "\(.number) [\(.labels | map(.name) | join(","))] \(.title)"'` with appropriate `--label` and `--state` filters.
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **Close**: `gh issue close <number> --comment "..."`

`gh` infers the repo from `git remote -v` automatically when run inside the clone, so no `--repo` flag is needed.

## Pull requests as a triage surface

**PRs as a request surface: no.** PRs are not a triage queue for this repo; `/triage` should ignore them.

If this ever changes to `yes`, external PRs run through the same labels and states as issues, using the `gh pr` equivalents (`gh pr view`, `gh pr list --state open --json ...` filtered to `authorAssociation` of `CONTRIBUTOR`/`FIRST_TIME_CONTRIBUTOR`/`NONE`, `gh pr comment`, `gh pr edit --add-label`, `gh pr close`). GitHub shares one number space across issues and PRs, so a bare `#42` may be either — resolve with `gh pr view 42`, falling back to `gh issue view 42`.

## When a skill says "publish to the issue tracker"

Create a GitHub issue with `gh issue create`.

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> --comments`.
