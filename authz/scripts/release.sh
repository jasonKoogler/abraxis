#!/bin/bash
# Release script for Authz using Git Flow branching model

set -e

# Function to display usage
function usage {
  echo "Usage: $0 [version]"
  echo ""
  echo "Examples:"
  echo "  $0 0.1.0        - Release version 0.1.0"
  echo "  $0 1.0.0-alpha  - Release version 1.0.0-alpha"
  exit 1
}

# Function to check if branch exists
function branch_exists {
  git show-ref --verify --quiet "refs/heads/$1"
  return $?
}

# Function to check if we're on the expected branch
function check_current_branch {
  local expected=$1
  local current=$(git rev-parse --abbrev-ref HEAD)
  if [ "$current" != "$expected" ]; then
    echo "Error: Not on $expected branch. Current branch is $current."
    echo "Please checkout the $expected branch first."
    exit 1
  fi
}

# Function to check for uncommitted changes
function check_clean_work_tree {
  if ! git diff-index --quiet HEAD --; then
    echo "Error: You have uncommitted changes. Please commit or stash them before proceeding."
    exit 1
  fi
}

# Check if version is provided
if [ -z "$1" ]; then
  echo "Error: Version is required"
  usage
fi

VERSION=$1

# Extract version components
if [[ $VERSION =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-([a-zA-Z0-9.-]+))?$ ]]; then
  MAJOR="${BASH_REMATCH[1]}"
  MINOR="${BASH_REMATCH[2]}"
  PATCH="${BASH_REMATCH[3]}"
  PRERELEASE="${BASH_REMATCH[5]}"
else
  echo "Error: Invalid version format. Must be MAJOR.MINOR.PATCH[-PRERELEASE]"
  exit 1
fi

# Ensure we're on dev branch and it's clean
check_current_branch "dev"
check_clean_work_tree

echo "Preparing release v$VERSION"

# Create a release branch
RELEASE_BRANCH="release/v$VERSION"
if branch_exists "$RELEASE_BRANCH"; then
  echo "Warning: Release branch $RELEASE_BRANCH already exists."
  read -p "Do you want to use the existing branch? (y/n): " use_existing
  if [ "$use_existing" != "y" ]; then
    echo "Release preparation aborted."
    exit 1
  fi
  git checkout "$RELEASE_BRANCH"
else
  echo "Creating release branch $RELEASE_BRANCH..."
  git checkout -b "$RELEASE_BRANCH"
fi

# Update version.go
echo "Updating version.go..."
cat > version.go << EOF
package authz

// Version information
const (
	// Version is the current version of the library
	Version = "$MAJOR.$MINOR.$PATCH"
	
	// VersionPrerelease is a pre-release marker for the version
	// If this is "" (empty string) then it means that it is a final release.
	// Otherwise, this is a pre-release such as "dev", "beta", "alpha", etc.
	VersionPrerelease = "$PRERELEASE"
)

// GetVersion returns the full version string
func GetVersion() string {
	if VersionPrerelease != "" {
		return Version + "-" + VersionPrerelease
	}
	return Version
}
EOF

# Update CHANGELOG.md
echo "Updating CHANGELOG.md..."
TODAY=$(date +%Y-%m-%d)
sed -i "s/## \[Unreleased\]/## [Unreleased]\n\n## [$MAJOR.$MINOR.$PATCH]${PRERELEASE:+"-$PRERELEASE"} - $TODAY/" CHANGELOG.md

# Run tests
echo "Running tests..."
go test ./...

# Commit changes
echo "Committing changes..."
git add version.go CHANGELOG.md
git commit -m "Release v$VERSION"

# Merge to master
echo "Merging to master branch..."
git checkout master
git merge --no-ff "$RELEASE_BRANCH" -m "Merge release v$VERSION into master"

# Tag the release on master
echo "Tagging release on master..."
git tag -a "v$VERSION" -m "Release v$VERSION"

# Push master and tags
echo "Pushing master branch and tags..."
git push origin master
git push origin "v$VERSION"

# Merge back to dev
echo "Merging changes back to dev branch..."
git checkout dev
git merge --no-ff "$RELEASE_BRANCH" -m "Merge release v$VERSION back to dev"

# Push dev branch
echo "Pushing dev branch..."
git push origin dev

echo ""
echo "Release v$VERSION completed successfully!"
echo ""
echo "Summary of actions:"
echo "1. Created release branch $RELEASE_BRANCH"
echo "2. Updated version.go and CHANGELOG.md"
echo "3. Merged to master and tagged as v$VERSION"
echo "4. Merged changes back to dev"
echo "5. Pushed all changes to remote repository"
echo ""
echo "Next steps:"
echo "- You can delete the release branch if no longer needed: git branch -d $RELEASE_BRANCH"
echo "- To delete it on remote: git push origin --delete $RELEASE_BRANCH"
echo "" 