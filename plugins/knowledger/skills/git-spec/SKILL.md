---
name: git-spec
description: Use when the user gives a git URL and says "添加/clone/添加规则库/添加 git spec", OR when the user says "更新/同步/pull spec/规则库". Handles cloning a new git rule set as a kb-type spec and pulling updates for existing spec-git specs, always followed by reindexing.
version: 1.0.0
triggers:
  - "添加规则库"
  - "添加 git spec"
  - "clone 这个 spec"
  - "拉取规则"
  - "更新 spec"
  - "同步 spec"
  - "pull spec"
  - "git spec"
  - "add git spec"
  - "pull specification"
  - "update specification"
---

# Git Spec Skill

Analog to `git-knowledge`, but for **rule sets** — a git repo becomes a kb-type specification that participates in `run_lint`.

## Two Scenarios

### 1. Add a New Git Spec

**Trigger**: user provides a git URL + intent to pull/clone/add it as a rule set / spec.

**Steps**:
1. Call `spec_git_add` with the URL (and optional `id`, `name`, `tags`, `scope`). This clones the repo, registers a text KB, and creates a kb-type spec pointing at the KB.
2. Call `index_knowledge` with the returned KB's `id` and `scope` to index the rule content.
3. Report success: spec cloned and indexed.

**Example user messages**: "添加这个规则库 https://github.com/…", "clone 这个 spec", "add this git rule set"

---

### 2. Update Existing Git Spec(s)

**Trigger**: user says "更新 spec", "同步 spec", "pull spec", etc.

#### Case A — user specifies a spec name/id

1. Call `spec_git_pull` with that `id` (and `scope` if specified).
2. Call `index_knowledge` with that `id` and `scope`.
3. Report success.

#### Case B — no specific spec mentioned

1. Call `spec_git_list` to discover all spec-git specs (global + project).
2. For each result, call `spec_git_pull` with its `id` and `scope`.
3. After all pulls, call `index_knowledge` for each successfully pulled spec.
4. Report a summary.

---

## Tool Reference

| Tool | When to use |
|------|-------------|
| `spec_git_add` | Clone a git repo as a new kb-type spec |
| `spec_git_pull` | Pull updates for an existing spec-git spec |
| `spec_git_list` | List all spec-git specs (global + project dirs) |
| `index_knowledge` | Reindex the backing KB after clone/pull |

Always reindex after every clone or pull — otherwise new/changed rules won't be searchable and `run_lint` may return stale results.
