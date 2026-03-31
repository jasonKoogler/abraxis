# Contributing to Authz

Thank you for your interest in contributing to Authz! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and considerate of others when contributing to this project.

## Branching Model

This project follows the Git Flow branching model:

- `master` - Contains production-ready code. All releases are merged into master and tagged.
- `dev` - Main development branch where features are integrated.
- `feature/*` - Feature branches for new functionality.
- `release/v*` - Release branches for preparing new releases.
- `hotfix/v*` - Hotfix branches for urgent fixes to production code.

### Branch Workflow

1. **Features**:

   - Features are developed in `feature/*` branches created from `dev`
   - When complete, features are merged back into `dev`

2. **Releases**:

   - When `dev` is ready for release, a `release/v*` branch is created
   - Only bug fixes and release preparations happen in release branches
   - When ready, the release branch is merged into both `master` and `dev`

3. **Hotfixes**:
   - Critical bugs in production are fixed in `hotfix/v*` branches from `master`
   - When complete, hotfixes are merged into both `master` and `dev`

## How to Contribute

1. **Fork the repository**

2. **Clone your fork**:

   ```
   git clone https://github.com/YOUR-USERNAME/authz.git
   cd authz
   ```

3. **Set up Git Flow**:

   ```
   make git-flow-init
   ```

4. **Create a feature branch**:

   ```
   make feature-start name=your-feature-name
   ```

   Or manually:

   ```
   git checkout dev
   git checkout -b feature/your-feature-name
   ```

5. **Make your changes**

6. **Run tests**:

   ```
   go test ./...
   ```

   Or:

   ```
   make test
   ```

7. **Commit your changes**:

   ```
   git commit -m "Add feature X"
   ```

8. **Push to your branch**:

   ```
   git push origin feature/your-feature-name
   ```

9. **Finish your feature**:
   ```
   make feature-finish name=your-feature-name
   ```
   Or create a Pull Request to merge into the `dev` branch

## Pull Request Guidelines

- Ensure your code passes all tests
- Update documentation if necessary
- Add tests for new features
- Follow the existing code style
- Keep pull requests focused on a single topic
- Reference any related issues in your PR description

## Development Setup

1. **Clone the repository**:

   ```
   git clone https://github.com/jasonKoogler/authz.git
   cd authz
   ```

2. **Install dependencies**:

   ```
   go mod download
   ```

3. **Run tests**:
   ```
   go test ./...
   ```

## Versioning

This project follows [Semantic Versioning](https://semver.org/).

## Release Process

For information about the release process, please see [RELEASE.md](RELEASE.md).
