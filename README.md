# og — overlays + git worktrees

Parallel branch development with isolated workspaces — backed by overlayfs
and git worktrees for minimal disk usage.

## What it does

`og` creates per-branch workspaces that share a read-only base (source +
dependencies) via overlayfs.  Only the diffs you make are copied to the
upper layer.  Git branches stay fully independent thanks to worktrees.

A typical `~/projects/demo` might look like:

```
demo/               ← base repo (locked read-only)
.og/workspaces/
  feat-auth/        ← og add feat-auth -b feat/auth
  fix-login/        ← og add fix-login -b fix/login
```

Each workspace **shares** `node_modules`, `target/`, and all source files with the
base.  Changed files are COW'd into the upper directory, so a 500 MB project
with two workspaces might use ~510 MB total.

## Quick start

```bash
# 1. Mark current repo as base (one-time)
cd ~/projects/demo
og init

# 2. Create a workspace
og add feature-x -b feat/x      # new branch
og add hotfix main               # existing branch

# 3. Enter and work
og feature-x
# or run a single command
og feature-x npm test

# 4. Check what's running
og ls

# 5. Clean up when done
og rm feature-x
```

## Commands

### `og init`

Run once per repo.  Marks the current directory as the base:

- Records path in `~/.og/base`
- Installs sudoers rules for passwordless mount/umount
- Installs a systemd user service for auto-resume on reboot
- Adds `/.og/` to `.git/info/exclude`
- Fingerprints the base for dependency change detection
- Locks the base (read-only bind mount; `.git` stays writable)

### `og add <name> [-b <branch>]`

Create an overlay workspace.  Second positional arg = existing branch;
`-b` = create a new branch.

```bash
og add fix-oauth fix/oauth    # checkout existing branch
og add rewrite   -b feat/rewrite  # create new branch
```

### `og <name> [cmd...]`

Enter a workspace interactively (spawns `$SHELL`), or run a single command.

```bash
og fix-oauth              # interactive shell
og fix-oauth make build   # run a command
```

Shortcut for `og enter <name>`.

### `og ls`

List all workspaces, their branches, and mount status.

### `og rm <name>`

Full cleanup: unmounts the overlay, removes the git worktree, deletes the
upper data.  Commit or push before removing.

### `og base lock|unlock`

The base is normally locked (read-only).  Unlock it when you need to pull
changes or update dependencies:

```bash
og base unlock
git pull
npm install
# base auto-locks on next og ls or og add
```

## How it works

```
Workspace overlay:
  upper/          ← your changes (writable)
  ───── OVERLAYFS
  lower/          ← base repo (read-only bind mount)
```

```
Git layer:
  base/           ← main worktree, shared object store
  .og/workspaces/<name>/.git  ← linked worktree, independent branch
```

- **overlayfs** merges the base (lower) and your workspace (upper).  Reads
  from the base when unchanged; writes land in the upper layer.
- **git worktrees** share the base's object database — no `.git` duplication.
- **lock** prevents accidental base modification via a read-only bind mount.

## Requirements

- Linux kernel with overlayfs support
- `sudo` access (for mount/umount; configured by `og init`)
- systemd (for auto-resume service)

## Install

```bash
go install github.com/linrunxin/og@latest
```
