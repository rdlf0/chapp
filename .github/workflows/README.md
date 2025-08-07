# GitHub Actions Workflows

This directory contains GitHub Actions workflows for the Chapp project.

## PR Validation Workflow

The `pr-validation.yml` workflow validates pull requests by running tests and builds.

### Triggers

- **Automatic**: PR events (opened, reopened, synchronize)

### Features

- ✅ **Test**: Runs all tests with coverage reporting
- ✅ **Build**: Compiles both servers for multiple platforms
- ✅ **Job Summaries**: Displays test coverage and build information
- ✅ **Multi-platform**: Builds for Linux, macOS, and Windows
- ✅ **Concurrency Control**: Cancels in-progress runs when new commits are pushed

### Output

- **Test Results**: Pass/fail status with coverage information
- **Build Results**: Success/failure status for all target platforms
- **Job Summaries**: Detailed coverage and build summaries in GitHub UI

### Usage

1. **Automatic**: Created automatically when PRs are opened, reopened, or updated
2. **Status**: Check PR status checks to see validation results

## Label Validation Workflow

The `label-validation.yml` workflow validates that PRs have exactly one release label.

### Triggers

- **Automatic**: PR events (opened, reopened, labeled, unlabeled, synchronize)

### Features

- ✅ **Label Validation**: Ensures PRs have exactly one release label
- ✅ **Required Labels**: Validates against `release: major`, `release: minor`, `release: patch`
- ✅ **Exact Count**: Requires exactly 1 label (not 0, not 2+)
- ✅ **Real-time Feedback**: Validates immediately when labels are added/removed

### Valid Labels

- **`release: major`**: For major version releases
- **`release: minor`**: For minor version releases  
- **`release: patch`**: For patch version releases

### Output

- **Pass**: PR has exactly one of the required release labels
- **Fail**: PR has 0 or 2+ release labels

### Usage

1. **Automatic**: Runs automatically when PRs are created or labels are modified
2. **Required**: PRs must have exactly one release label to pass validation
3. **Status**: Check PR status checks to see label validation results

## Release Workflow

The `release.yml` workflow automatically builds, tests, and releases the Chapp application.

### Triggers

- **Automatic**: Merged PRs to the `master` branch (version type determined by PR labels)
- **Manual**: Manual workflow dispatch with version type selection

### Features

- ✅ **Test**: Runs all tests with coverage reporting
- ✅ **Build**: Compiles both servers for multiple platforms
- ✅ **Semantic Versioning**: Automatically increments version numbers
- ✅ **PR Label Versioning**: Determines version type from PR labels
- ✅ **GitHub Releases**: Creates releases with binaries attached
- ✅ **Manual Control**: Supports manual triggering with version type selection
- ✅ **Job Summaries**: Displays test coverage and release information
- ✅ **Multi-platform**: Builds for Linux, macOS, and Windows

### Version Types

The workflow determines the version type based on the trigger:

#### For Merged PRs (Automatic)
- **`release: major`**: Increments major version (0.1.0 → 1.0.0)
- **`release: minor`**: Increments minor version (0.1.0 → 0.2.0) - **Default**
- **`release: patch`**: Increments patch version (0.1.0 → 0.1.1)

#### For Manual Triggers
- **minor**: Increments minor version (0.1.0 → 0.2.0) - **Default**
- **patch**: Increments patch version (0.1.0 → 0.1.1)
- **major**: Increments major version (0.1.0 → 1.0.0)

### Output

- **Git Tags**: Semantic version tags (v0.1.0, v0.1.1, etc.)
- **GitHub Releases**: Release notes and downloadable binaries
- **Build Artifacts**: Static and WebSocket server binaries for multiple platforms

### Usage

1. **Automatic Release**: 
   - Create a PR to `master` branch
   - Add one of the release labels: `release: major`, `release: minor`, or `release: patch`
   - Merge the PR (workflow will trigger automatically)

2. **Manual Release**: Go to Actions → Release → Run workflow → Select version type

### Requirements

- Go 1.24.5+
- GitHub CLI (for releases)
- Write permissions to repository

### Optional: GPG Signing

To enable signed tags and commits, add these secrets to your repository:

- **`GPG_PRIVATE_KEY`**: Your GPG private key (armored format)
- **`GPG_KEY_ID`**: Your GPG key ID (e.g., `ABC123DEF456`)

**To export your GPG key:**
```bash
# Export private key (armored)
gpg --export-secret-key --armor your-email@example.com

# Get key ID
gpg --list-secret-keys --keyid-format LONG
``` 