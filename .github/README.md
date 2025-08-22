# GitHub Actions Workflows

This directory contains GitHub Actions workflows for automated CI/CD, security scanning, and documentation generation.

## Workflows

### 🔄 CI Workflow (`ci.yml`)

**Triggers:**
- Push to `main` or `master` branch
- Pull requests to `main` or `master` branch

**Jobs:**
- **Test**: Runs formatting checks, linting, builds, and tests
- **Build**: Creates release builds and uploads artifacts (only on push to main/master)

**Features:**
- Installs Rust stable and nightly toolchains
- Installs system dependencies (clang, llvm, libelf, etc.)
- Caches cargo registry, index, and build artifacts
- Runs `cargo fmt --check`, `cargo clippy`, `cargo build`, and `cargo test`
- Uploads release artifacts for successful builds

### 🚀 Release Workflow (`release.yml`)

**Triggers:**
- Push of tags matching `v*` pattern (e.g., `v1.0.0`)

**Jobs:**
- **Create Release**: Creates a GitHub release
- **Build Release**: Builds binaries for multiple platforms

**Supported Platforms:**
- `x86_64-unknown-linux-gnu` (Intel/AMD 64-bit Linux)
- `aarch64-unknown-linux-gnu` (ARM 64-bit Linux)

**Artifacts:**
- Compressed tar.gz files containing the binary
- Automatically attached to the GitHub release

### 🔒 Security Workflow (`security.yml`)

**Triggers:**
- Push to `main` or `master` branch
- Pull requests to `main` or `master` branch
- Weekly schedule (Sundays at midnight UTC)

**Jobs:**
- **Security Audit**: Runs `cargo audit` to check for known vulnerabilities
- **Dependency Review**: Reviews dependencies in pull requests

**Tools:**
- `cargo-audit`: Scans for security vulnerabilities
- `cargo-deny`: Checks licenses and banned dependencies
- GitHub's dependency review action

### 📚 Documentation Workflow (`docs.yml`)

**Triggers:**
- Push to `main` or `master` branch
- Pull requests to `main` or `master` branch

**Jobs:**
- **Generate Documentation**: Creates Rust documentation with `cargo doc`
- **README Sync**: Validates README files consistency

**Features:**
- Generates and deploys documentation to GitHub Pages
- Checks that both English and Chinese README files exist
- Validates that example flame graph is referenced in both READMEs

## Configuration Files

### `deny.toml`

Configuration for `cargo-deny` tool that defines:
- **Licenses**: Allowed and denied license types
- **Advisories**: Security vulnerability handling
- **Bans**: Banned crates and multiple version handling
- **Sources**: Allowed registries and git sources

## Usage

### Creating a Release

1. Update version in `Cargo.toml` files
2. Update `CHANGELOG_EN.md` and `CHANGELOG_ZH.md`
3. Commit changes
4. Create and push a tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
5. The release workflow will automatically:
   - Create a GitHub release
   - Build binaries for supported platforms
   - Upload artifacts to the release

### Local Testing

Before pushing, you can run the same checks locally:

```bash
# Format check
cargo fmt --all -- --check

# Linting
cargo clippy --all-targets --all-features -- -D warnings

# Build
cargo build --verbose

# Tests
cargo test --verbose

# Security audit
cargo install cargo-audit
cargo audit

# Dependency check
cargo install cargo-deny
cargo deny check

# Documentation
cargo doc --no-deps --all-features
```

### Secrets and Permissions

The workflows use the default `GITHUB_TOKEN` which is automatically provided by GitHub Actions. No additional secrets are required.

For the documentation workflow to deploy to GitHub Pages, ensure that:
1. GitHub Pages is enabled in repository settings
2. Source is set to "GitHub Actions"

## Troubleshooting

### Common Issues

1. **eBPF Build Failures**: Ensure Linux headers are available and nightly Rust is installed
2. **Cross-compilation Issues**: Check that target-specific linkers are properly configured
3. **Documentation Deployment**: Verify GitHub Pages settings and permissions
4. **Security Audit Failures**: Review and address reported vulnerabilities or add exceptions to `deny.toml`

### Debugging

- Check workflow logs in the "Actions" tab of your GitHub repository
- Enable debug logging by setting `ACTIONS_STEP_DEBUG` secret to `true`
- Test workflows locally using [act](https://github.com/nektos/act)