# Contributing to golang-profiling

[中文版本](CONTRIBUTING_zh.md) | English

Thank you for your interest in contributing to golang-profiling! This document provides guidelines and information for contributors.

## 🚀 Getting Started

### Prerequisites

- Linux system (kernel 4.4+)
- Rust toolchain (stable and nightly)
- System dependencies (clang, llvm, libelf, etc.)
- Git

### Development Setup

1. **Fork and Clone**
   ```bash
   git clone https://github.com/YOUR_USERNAME/golang-profile.git
   cd golang-profile
   ```

2. **Install Dependencies**
   ```bash
   # Install Rust
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
   source ~/.cargo/env
   
   # Install nightly for eBPF
   rustup install nightly
   rustup component add rust-src --toolchain nightly
   
   # Install system dependencies (Ubuntu/Debian)
   sudo apt update
   sudo apt install -y clang llvm libelf-dev libz-dev pkg-config linux-headers-$(uname -r) perl
   
   # Install bpf-linker
   cargo install bpf-linker
   ```

3. **Build and Test**
   ```bash
   cargo build
   cargo test
   ```

## 🔄 Development Workflow

### Code Quality

We use automated checks via GitHub Actions. Before submitting a PR, ensure your code passes:

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
```

### Commit Guidelines

- Use clear, descriptive commit messages
- Follow conventional commits format when possible:
  - `feat:` for new features
  - `fix:` for bug fixes
  - `docs:` for documentation changes
  - `refactor:` for code refactoring
  - `test:` for test additions/modifications
  - `ci:` for CI/CD changes

### Pull Request Process

1. **Create a Feature Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Changes**
   - Write clean, well-documented code
   - Add tests for new functionality
   - Update documentation as needed

3. **Test Locally**
   ```bash
   cargo fmt
   cargo clippy --fix
   cargo test
   ```

4. **Submit Pull Request**
   - Push your branch to your fork
   - Create a pull request with a clear description
   - Link any related issues

## 🏗️ Project Structure

```
golang-profile/
├── .github/                 # GitHub Actions workflows
│   ├── workflows/          # CI/CD configurations
│   └── README.md           # Workflows documentation
├── golang-profiling/       # Main application
│   ├── src/               # Rust source code
│   └── build.rs           # Build script
├── golang-profiling-ebpf/  # eBPF programs
│   └── src/               # eBPF source code
├── golang-profiling-common/ # Shared code
├── example/               # Example Go application
├── flamegraph.pl          # Flame graph generation script
└── docs/                  # Documentation
```

## 🔧 GitHub Actions

Our CI/CD pipeline includes:

### Continuous Integration (`ci.yml`)
- **Triggers**: Push/PR to main branch
- **Checks**: Format, lint, build, test
- **Artifacts**: Release binaries (on main branch)

### Security Audit (`security.yml`)
- **Triggers**: Push/PR to main branch, weekly schedule
- **Checks**: Vulnerability scan, dependency review

### Release (`release.yml`)
- **Triggers**: Version tags (`v*`)
- **Outputs**: Multi-platform binaries, GitHub releases

### Documentation (`docs.yml`)
- **Triggers**: Push/PR to main branch
- **Outputs**: API docs, GitHub Pages deployment

## 🐛 Bug Reports

When reporting bugs, please include:

- **Environment**: OS version, kernel version, Rust version
- **Steps to reproduce**: Clear, minimal reproduction steps
- **Expected behavior**: What should happen
- **Actual behavior**: What actually happens
- **Logs**: Relevant error messages or logs
- **Additional context**: Any other relevant information

## 💡 Feature Requests

For feature requests:

- **Use case**: Describe the problem you're trying to solve
- **Proposed solution**: Your idea for addressing it
- **Alternatives**: Other solutions you've considered
- **Additional context**: Any other relevant information

## 📝 Documentation

Documentation improvements are always welcome:

- **Code comments**: Explain complex logic
- **README updates**: Keep installation and usage instructions current
- **API documentation**: Document public interfaces
- **Examples**: Add usage examples and tutorials

## 🔒 Security

For security-related issues:

- **Do not** open public issues for security vulnerabilities
- Contact maintainers privately
- Provide detailed information about the vulnerability
- Allow time for fixes before public disclosure

## 📄 License

By contributing, you agree that your contributions will be licensed under the same terms as the project (MIT/Apache-2.0).

## 🤝 Code of Conduct

Please be respectful and constructive in all interactions. We're committed to providing a welcoming and inclusive environment for all contributors.

## 📞 Getting Help

- **Issues**: Use GitHub issues for bugs and feature requests
- **Discussions**: Use GitHub discussions for questions and ideas
- **Documentation**: Check the README and documentation first

## 🎯 Areas for Contribution

We especially welcome contributions in these areas:

- **Performance optimizations**: Improve profiling efficiency
- **Platform support**: Add support for more architectures
- **Visualization**: Enhance flame graph features
- **Documentation**: Improve guides and examples
- **Testing**: Add more comprehensive tests
- **CI/CD**: Improve automation and workflows

Thank you for contributing to golang-profiling! 🚀