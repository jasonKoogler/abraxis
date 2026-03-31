# Release Process for Authz

This document outlines the process for creating a new release of the Authz library using the Git Flow branching model.

## Branching Strategy

This project follows the Git Flow branching model. For detailed information about our branching strategy, please see [BRANCHING.md](BRANCHING.md).

## Release Process Overview

1. Development happens on the `dev` branch and feature branches
2. When ready for release, a release branch is created
3. After testing and finalization, the release is merged to `master` and tagged
4. Changes are merged back to `dev` for continued development

## Creating a Release

### Using the Makefile

All releases must follow the Git Flow branching model. The Makefile provides several convenient targets to ensure this:

```bash
# Create a specific version release
make release version=1.2.3

# Create a patch release (automatically increments the patch version)
make release-patch

# Create a minor release (automatically increments the minor version)
make release-minor

# Create a major release (automatically increments the major version)
make release-major
```

These commands will:

1. Verify you are on the `dev` branch
2. Check for uncommitted changes
3. Create a release branch from `dev`
4. Update version information
5. Update the changelog
6. Run tests
7. Merge to `master` and tag the release
8. Merge changes back to `dev`
9. Push all changes to the remote repository

### Starting Next Development Cycle

After a release, you can prepare for the next development cycle:

```bash
# Prepare for next patch development cycle
make next-dev-patch

# Prepare for next minor development cycle
make next-dev-minor

# Prepare for next major development cycle
make next-dev-major
```

These commands will also verify you are on the `dev` branch and have no uncommitted changes.

## Hotfix Process

For critical bugs in production that need immediate fixes:

```bash
# Start a hotfix
make hotfix-start version=1.2.4

# After making changes, finish the hotfix
make hotfix-finish version=1.2.4
```

This will:

1. Create a hotfix branch from `master`
2. After fixes are made, merge to `master` and tag
3. Merge changes back to `dev`
4. Push all changes to the remote repository

## Git Flow Release Process Details

For those who want to understand the underlying process, here's what happens during a release:

1. **Start on the dev branch**:
   The release process always starts from the `dev` branch.

2. **Create a release branch**:
   A branch named `release/vX.Y.Z` is created from `dev`.

3. **Update version files**:
   Version information is updated in `version.go` and the changelog.

4. **Testing and fixes**:
   Any final testing and bug fixes happen on the release branch.

5. **Merge to master**:
   The release branch is merged to `master` using `--no-ff` to preserve history.

6. **Tag the release**:
   A tag is created on `master` for the release.

7. **Merge back to dev**:
   Changes are merged back to `dev` to ensure continued development includes the release changes.

8. **Clean up**:
   The release branch can be deleted after the release is complete.

## Post-Release Tasks

After completing a release:

1. **Documentation**:

   - Update internal documentation with new features and changes
   - Notify relevant teams about the new release
   - Provide upgrade instructions if necessary

2. **Dependency Updates**:

   - Update dependent projects to use the new version
   - Use specific version references in `go.mod` files

3. **Monitoring**:
   - Monitor usage of the new version in internal systems
   - Be prepared to address any issues that arise
