#!/bin/bash
# Script to prepare for the next development cycle using Git Flow

set -e

# Function to display usage
function usage {
  echo "Usage: $0 [major|minor|patch]"
  echo ""
  echo "Examples:"
  echo "  $0 minor  - Prepare for next minor version"
  echo "  $0 patch  - Prepare for next patch version"
  echo ""
  echo "Note: This script must be run on the dev branch as part of the Git Flow process."
  exit 1
}

# Function to check if we're on the expected branch
function check_current_branch {
  local expected=$1
  local current=$(git rev-parse --abbrev-ref HEAD)
  if [ "$current" != "$expected" ]; then
    echo "Error: Not on $expected branch. Current branch is $current."
    echo "Please checkout the $expected branch first."
    echo "This script must be run on the dev branch as part of the Git Flow process."
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

# Check if component is provided
if [ -z "$1" ]; then
  echo "Error: Version component is required"
  usage
fi

COMPONENT=$1

# Validate component
if [[ ! "$COMPONENT" =~ ^(major|minor|patch)$ ]]; then
  echo "Error: Invalid component. Must be major, minor, or patch"
  usage
fi

# Ensure we're on dev branch and it's clean
check_current_branch "dev"
check_clean_work_tree

echo "Following Git Flow: preparing next development cycle on dev branch"

# Get current version from version.go
CURRENT_VERSION=$(grep "Version = " version.go | cut -d'"' -f2)

# Parse version components
if [[ $CURRENT_VERSION =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  MAJOR="${BASH_REMATCH[1]}"
  MINOR="${BASH_REMATCH[2]}"
  PATCH="${BASH_REMATCH[3]}"
else
  echo "Error: Could not parse current version: $CURRENT_VERSION"
  exit 1
fi

# Calculate next version
case $COMPONENT in
  major)
    NEXT_MAJOR=$((MAJOR + 1))
    NEXT_MINOR=0
    NEXT_PATCH=0
    ;;
  minor)
    NEXT_MAJOR=$MAJOR
    NEXT_MINOR=$((MINOR + 1))
    NEXT_PATCH=0
    ;;
  patch)
    NEXT_MAJOR=$MAJOR
    NEXT_MINOR=$MINOR
    NEXT_PATCH=$((PATCH + 1))
    ;;
esac

NEXT_VERSION="$NEXT_MAJOR.$NEXT_MINOR.$NEXT_PATCH"
echo "Preparing for next development cycle: v$NEXT_VERSION-dev"

# Update version.go
echo "Updating version.go..."
cat > version.go << EOF
package authz

// Version information
const (
	// Version is the current version of the library
	Version = "$NEXT_VERSION"
	
	// VersionPrerelease is a pre-release marker for the version
	// If this is "" (empty string) then it means that it is a final release.
	// Otherwise, this is a pre-release such as "dev", "beta", "alpha", etc.
	VersionPrerelease = "dev"
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
sed -i '1s/^/# Changelog\n\n## [Unreleased]\n\n/' CHANGELOG.md

# Commit changes
echo "Committing changes..."
git add version.go CHANGELOG.md
git commit -m "Begin development on v$NEXT_VERSION"

# Push changes
echo "Pushing changes to dev branch..."
git push origin dev

echo ""
echo "Next development cycle v$NEXT_VERSION-dev prepared successfully!"
echo ""
echo "Summary of actions:"
echo "1. Updated version.go to v$NEXT_VERSION-dev"
echo "2. Added new Unreleased section to CHANGELOG.md"
echo "3. Committed and pushed changes to dev branch"
echo ""
echo "Git Flow process complete. Development will continue on the dev branch."
echo "" 