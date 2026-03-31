#!/bin/bash
# Script to help with versioning the Authz library using Git Flow

set -e

# Create directory if it doesn't exist
mkdir -p scripts

# Function to display usage
function usage {
  echo "Usage: $0 [command] [options]"
  echo ""
  echo "Commands:"
  echo "  bump [major|minor|patch]  - Bump version number"
  echo ""
  echo "Examples:"
  echo "  $0 bump minor             - Bump minor version"
  exit 1
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

# Function to get current version
function get_current_version {
  grep "Version = " version.go | cut -d'"' -f2
}

# Function to get current pre-release
function get_current_prerelease {
  grep "VersionPrerelease = " version.go | cut -d'"' -f2
}

# Function to update version in version.go
function update_version {
  local version=$1
  local prerelease=$2
  
  # Update Version
  sed -i "s/Version = \".*\"/Version = \"$version\"/" version.go
  
  # Update VersionPrerelease
  sed -i "s/VersionPrerelease = \".*\"/VersionPrerelease = \"$prerelease\"/" version.go
  
  echo "Updated version.go to version $version${prerelease:+"-$prerelease"}"
}

# Function to bump version
function bump_version {
  local current_version=$(get_current_version)
  local major=$(echo $current_version | cut -d. -f1)
  local minor=$(echo $current_version | cut -d. -f2)
  local patch=$(echo $current_version | cut -d. -f3)
  
  case $1 in
    major)
      major=$((major + 1))
      minor=0
      patch=0
      ;;
    minor)
      minor=$((minor + 1))
      patch=0
      ;;
    patch)
      patch=$((patch + 1))
      ;;
    *)
      echo "Invalid version component: $1"
      usage
      ;;
  esac
  
  echo "$major.$minor.$patch"
}

# Main command processing
case $1 in
  bump)
    if [ -z "$2" ]; then
      echo "Error: Missing version component"
      usage
    fi
    
    # Ensure we're on dev branch and it's clean
    check_current_branch "dev"
    check_clean_work_tree
    
    current_version=$(get_current_version)
    current_prerelease=$(get_current_prerelease)
    new_version=$(bump_version $2)
    
    update_version "$new_version" "$current_prerelease"
    
    echo "Version bumped to $new_version${current_prerelease:+"-$current_prerelease"}"
    echo "This change is only local. To create a release, use one of the release targets in the Makefile."
    ;;
    
  *)
    usage
    ;;
esac 