# Git Flow Branching Model

This document describes the Git Flow branching model used in the Authz project.

## Overview

We use a modified version of the Git Flow branching model to manage our development process. This model helps us maintain a clean repository history and enables parallel development.

## Branch Structure

Our repository has the following main branches:

- **`master`**: The production branch containing the latest stable release.
- **`dev`**: The main development branch where features are integrated.

And the following supporting branches:

- **`feature/*`**: Feature branches for developing new functionality.
- **`release/v*`**: Release branches for preparing new releases.
- **`hotfix/v*`**: Hotfix branches for urgent fixes to production code.

## Workflow

### Initialization

To initialize the Git Flow structure in a new clone of the repository:

```bash
make git-flow-init
```

This will ensure both `master` and `dev` branches exist and set you up on the `dev` branch.

### Feature Development

1. **Start a new feature**:

   ```bash
   make feature-start name=my-feature
   ```

   This creates a new branch `feature/my-feature` from `dev`.

2. **Work on your feature**:

   Make commits to your feature branch as you develop.

3. **Finish the feature**:

   ```bash
   make feature-finish name=my-feature
   ```

   This merges your feature branch back into `dev` using `--no-ff` to preserve history.

### Release Process

1. **Create a release**:

   When `dev` contains all features for a release, create a release branch:

   ```bash
   make release version=1.2.3
   ```

   This creates a `release/v1.2.3` branch from `dev`, updates version files, and prepares the release.

2. **Test and fix**:

   The release branch is where final testing happens. Only bug fixes should be committed to this branch.

3. **Finish the release**:

   The release script will:

   - Merge the release branch into `master`
   - Tag the release in `master`
   - Merge the release branch back into `dev`
   - Push all changes to the remote repository

### Hotfix Process

For critical bugs in production that can't wait for the next release:

1. **Start a hotfix**:

   ```bash
   make hotfix-start version=1.2.4
   ```

   This creates a `hotfix/v1.2.4` branch from `master`.

2. **Fix the bug**:

   Make commits to fix the issue.

3. **Finish the hotfix**:

   ```bash
   make hotfix-finish version=1.2.4
   ```

   This:

   - Merges the hotfix into `master`
   - Tags the new version
   - Merges the hotfix into `dev`
   - Pushes all changes

## Branch Naming Conventions

- Feature branches: `feature/descriptive-name`
- Release branches: `release/vX.Y.Z`
- Hotfix branches: `hotfix/vX.Y.Z`

## Commit Messages

Write clear, concise commit messages that explain what the commit does. Prefix with the type of change:

- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `style:` for formatting changes
- `refactor:` for code refactoring
- `test:` for adding or modifying tests
- `chore:` for maintenance tasks

Example: `feat: Add user authentication`

## Visual Representation

```
    ┌─────────────────────────────────────────┐
    │                                         │
    │                 master                  │
    │                                         │
    └───────────┬─────────────┬───────────────┘
                │             │
                │             │    ┌─────────────────┐
                │             │    │                 │
                │             └────┤  hotfix/v1.0.1  │
                │                  │                 │
                │                  └─────────────────┘
                │
    ┌───────────┴─────────────────────────────┐
    │                                         │
    │                  dev                    │
    │                                         │
    └─┬─────────────────┬────────────────────┬┘
      │                 │                    │
      │                 │                    │
┌─────┴──────┐  ┌───────┴────────┐  ┌────────┴───────┐
│            │  │                │  │                │
│ feature/a  │  │  release/v1.0  │  │  feature/b     │
│            │  │                │  │                │
└────────────┘  └────────────────┘  └────────────────┘
```

## Tools and Commands

Our Makefile provides several commands to help manage the Git Flow process:

- `make git-flow-init` - Initialize Git Flow branches
- `make feature-start name=X` - Start a new feature
- `make feature-finish name=X` - Finish a feature
- `make release version=X.Y.Z` - Create a release
- `make release-patch` - Create a patch release
- `make release-minor` - Create a minor release
- `make release-major` - Create a major release
- `make hotfix-start version=X.Y.Z` - Start a hotfix
- `make hotfix-finish version=X.Y.Z` - Finish a hotfix
- `make next-dev-patch` - Prepare for next patch development cycle
- `make next-dev-minor` - Prepare for next minor development cycle
- `make next-dev-major` - Prepare for next major development cycle

## Troubleshooting

### Common Issues

1. **Merge Conflicts**:

   - When finishing a feature or release, you might encounter merge conflicts
   - Resolve conflicts manually, then continue the merge

2. **Forgotten Changes**:

   - If you forgot to include changes in a release, create a new hotfix

3. **Accidental Commits**:
   - If you accidentally commit to the wrong branch, use `git cherry-pick` to move the commit

### Getting Help

If you encounter issues with the Git Flow process, please:

1. Check this documentation
2. Run `make help` to see available commands
3. Contact the repository maintainers
