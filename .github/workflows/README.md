# GitHub Actions Workflows

This directory contains GitHub Actions workflows for the Chapp project.

## Release Workflow

The `release.yml` workflow automatically builds, tests, and releases the Chapp application.

### Triggers

- **Automatic**: Merged PRs to the `master` branch
- **Manual**: Manual workflow dispatch with version type selection

### Features

- ✅ **Test**: Runs all tests with coverage reporting
- ✅ **Build**: Compiles both servers for multiple platforms
- ✅ **Semantic Versioning**: Automatically increments version numbers
- ✅ **GitHub Releases**: Creates releases with binaries attached
- ✅ **Manual Control**: Supports manual triggering with version type selection
- ✅ **Job Summaries**: Displays test coverage and release information
- ✅ **Multi-platform**: Builds for Linux, macOS, and Windows

### Version Types

- **minor**: Increments minor version (0.1.0 → 0.2.0) - **Default**
- **patch**: Increments patch version (0.1.0 → 0.1.1)
- **major**: Increments major version (0.1.0 → 1.0.0)

### Output

- **Git Tags**: Semantic version tags (v0.1.0, v0.1.1, etc.)
- **GitHub Releases**: Release notes and downloadable binaries
- **Build Artifacts**: Static and WebSocket server binaries for multiple platforms

### Usage

1. **Automatic Release**: Merge PR to `master` branch
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