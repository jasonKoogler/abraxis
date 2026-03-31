# Versioning Strategy for Authz (Private Repository)

This document outlines the versioning strategy for the Authz library as a private repository.

## Versioning Approach

Even as a private repository, we follow [Semantic Versioning (SemVer)](https://semver.org/) to maintain clarity and consistency:

- **MAJOR** version for incompatible API changes (v1.0.0, v2.0.0)
- **MINOR** version for new functionality in a backward compatible manner (v1.1.0, v1.2.0)
- **PATCH** version for backward compatible bug fixes (v1.0.1, v1.0.2)

## Release Process

We use a simplified release process with custom scripts:

### Creating a Release

Use the `scripts/release.sh` script to create a new release:

```bash
# Release version 0.1.0
./scripts/release.sh 0.1.0

# Release alpha version
./scripts/release.sh 1.0.0-alpha
```

This script will:

1. Update the version in `version.go`
2. Update the CHANGELOG.md
3. Run tests
4. Commit the changes
5. Create a git tag

After running the script, you'll need to push the changes:

```bash
git push origin master
git push origin v0.1.0  # Replace with your version
```

### Starting Next Development Cycle

Use the `scripts/next-dev.sh` script to start the next development cycle:

```bash
# Prepare for next minor version
./scripts/next-dev.sh minor

# Prepare for next patch version
./scripts/next-dev.sh patch
```

This script will:

1. Calculate the next version
2. Update version.go with the new version and "dev" suffix
3. Add a new "Unreleased" section to CHANGELOG.md
4. Commit the changes

After running the script, push the changes:

```bash
git push origin master
```

## Version Control

For a private repository, we use a simplified git workflow:

1. **Branches**

   - `master`: Always contains the latest stable release
   - `feature/*`: Feature branches for new functionality
   - `hotfix/*`: Branches for urgent fixes

2. **Tags**
   - Tag all releases with `v` prefix (e.g., `v1.0.0`)
   - Include pre-release designations in tags (e.g., `v1.0.0-alpha`)

## Dependency Management

For internal dependencies:

1. **Go Modules**

   - Use specific version references in `go.mod`
   - Example: `github.com/jasonKoogler/authz v1.0.0`

2. **Version Pinning**
   - Pin to exact versions for stability
   - Use version ranges only when necessary

## Communicating Changes

For a private repository, maintain clear communication about versions:

1. **CHANGELOG.md**

   - Document all changes in the CHANGELOG
   - Categorize changes as Added, Changed, Deprecated, Removed, Fixed, or Security

2. **Release Notes**
   - Create detailed release notes for each version
   - Highlight breaking changes prominently

## Stability Guarantees

Define clear stability guarantees:

1. **Alpha Releases** (`0.x.y-alpha`)

   - Experimental features
   - No stability guarantees
   - API may change without notice

2. **Beta Releases** (`0.x.y-beta` or `x.y.z-beta`)

   - Feature complete for the targeted release
   - API may still change
   - Not recommended for production

3. **Final Releases** (`x.y.z`)
   - Stable API
   - Production ready
   - Follows semantic versioning guarantees

## Handling Breaking Changes

For a private repository, breaking changes can be managed more flexibly:

1. **Coordinate with Consumers**

   - Communicate breaking changes in advance
   - Provide migration assistance

2. **Version Bumping**

   - Increment MAJOR version for breaking changes
   - Document all breaking changes prominently

3. **Deprecation Policy**
   - Mark features as deprecated before removal
   - Provide deprecation warnings in code
   - Allow for transition periods when possible
