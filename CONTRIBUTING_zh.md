# 贡献指南

[English](CONTRIBUTING.md) | 中文版本

感谢您对 golang-profiling 项目的关注！本文档为贡献者提供指南和相关信息。

## 🚀 快速开始

### 前置要求

- Linux 系统 (内核 4.4+)
- Rust 工具链 (stable 和 nightly)
- 系统依赖 (clang, llvm, libelf 等)
- Git

### 开发环境设置

1. **Fork 和克隆**
   ```bash
   git clone https://github.com/YOUR_USERNAME/golang-profile.git
   cd golang-profile
   ```

2. **安装依赖**
   ```bash
   # 安装 Rust
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
   source ~/.cargo/env
   
   # 安装 nightly 工具链用于 eBPF 开发
   rustup install nightly
   rustup component add rust-src --toolchain nightly
   
   # 安装系统依赖 (Ubuntu/Debian)
   sudo apt update
   sudo apt install -y clang llvm libelf-dev libz-dev pkg-config linux-headers-$(uname -r) perl
   
   # 安装 bpf-linker
   cargo install bpf-linker
   ```

3. **构建和测试**
   ```bash
   cargo build
   cargo test
   ```

## 🔄 开发工作流

### 代码质量

我们通过 GitHub Actions 进行自动化检查。在提交 PR 之前，请确保您的代码通过以下检查：

```bash
# 格式检查
cargo fmt --all -- --check

# 代码检查
cargo clippy --all-targets --all-features -- -D warnings

# 构建
cargo build --verbose

# 测试
cargo test --verbose

# 安全审计
cargo install cargo-audit
cargo audit

# 依赖检查
cargo install cargo-deny
cargo deny check
```

### 提交规范

- 使用清晰、描述性的提交信息
- 尽可能遵循约定式提交格式：
  - `feat:` 新功能
  - `fix:` 错误修复
  - `docs:` 文档更改
  - `refactor:` 代码重构
  - `test:` 测试添加/修改
  - `ci:` CI/CD 更改

### Pull Request 流程

1. **创建功能分支**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **进行更改**
   - 编写清晰、有良好文档的代码
   - 为新功能添加测试
   - 根据需要更新文档

3. **本地测试**
   ```bash
   cargo fmt
   cargo clippy --fix
   cargo test
   ```

4. **提交 Pull Request**
   - 将分支推送到您的 fork
   - 创建带有清晰描述的 pull request
   - 链接相关的 issues

## 🏗️ 项目结构

```
golang-profile/
├── .github/                 # GitHub Actions 工作流
│   ├── workflows/          # CI/CD 配置
│   └── README.md           # 工作流文档
├── golang-profiling/       # 主应用程序
│   ├── src/               # Rust 源代码
│   └── build.rs           # 构建脚本
├── golang-profiling-ebpf/  # eBPF 程序
│   └── src/               # eBPF 源代码
├── golang-profiling-common/ # 共享代码
├── example/               # 示例 Go 应用程序
├── flamegraph.pl          # 火焰图生成脚本
└── docs/                  # 文档
```

## 🔧 GitHub Actions

我们的 CI/CD 流水线包括：

### 持续集成 (`ci.yml`)
- **触发条件**: 推送/PR 到主分支
- **检查项**: 格式、检查、构建、测试
- **产物**: 发布二进制文件（主分支）

### 安全审计 (`security.yml`)
- **触发条件**: 推送/PR 到主分支，每周定时
- **检查项**: 漏洞扫描、依赖审查

### 发布 (`release.yml`)
- **触发条件**: 版本标签 (`v*`)
- **输出**: 多平台二进制文件、GitHub releases

### 文档 (`docs.yml`)
- **触发条件**: 推送/PR 到主分支
- **输出**: API 文档、GitHub Pages 部署

## 🐛 错误报告

报告错误时，请包含：

- **环境信息**: 操作系统版本、内核版本、Rust 版本
- **重现步骤**: 清晰、最小的重现步骤
- **期望行为**: 应该发生什么
- **实际行为**: 实际发生了什么
- **日志**: 相关的错误信息或日志
- **附加上下文**: 任何其他相关信息

## 💡 功能请求

对于功能请求：

- **用例**: 描述您试图解决的问题
- **建议解决方案**: 您解决问题的想法
- **替代方案**: 您考虑过的其他解决方案
- **附加上下文**: 任何其他相关信息

## 📝 文档

文档改进总是受欢迎的：

- **代码注释**: 解释复杂逻辑
- **README 更新**: 保持安装和使用说明的时效性
- **API 文档**: 记录公共接口
- **示例**: 添加使用示例和教程

## 🔒 安全

对于安全相关问题：

- **不要**为安全漏洞开启公开 issues
- 私下联系维护者
- 提供关于漏洞的详细信息
- 在公开披露之前给予修复时间

## 📄 许可证

通过贡献，您同意您的贡献将在与项目相同的条款下获得许可 (MIT/Apache-2.0)。

## 🤝 行为准则

请在所有互动中保持尊重和建设性。我们致力于为所有贡献者提供友好和包容的环境。

## 📞 获取帮助

- **Issues**: 使用 GitHub issues 报告错误和功能请求
- **Discussions**: 使用 GitHub discussions 提问和讨论想法
- **文档**: 首先查看 README 和文档

## 🎯 贡献领域

我们特别欢迎在以下领域的贡献：

- **性能优化**: 提高分析效率
- **平台支持**: 添加对更多架构的支持
- **可视化**: 增强火焰图功能
- **文档**: 改进指南和示例
- **测试**: 添加更全面的测试
- **CI/CD**: 改进自动化和工作流

感谢您为 golang-profiling 做出贡献！🚀