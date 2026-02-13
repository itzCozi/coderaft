# Contributing to coderaft

Thank you for your interest in contributing to coderaft! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [How to Contribute](#how-to-contribute)
- [Development Setup](#development-setup)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Continuous Integration](#continuous-integration)
- [Submitting Changes](#submitting-changes)
- [Reporting Issues](#reporting-issues)
- [Feature Requests](#feature-requests)
- [Documentation](#documentation)
- [Community](#community)
- [License](#license)

## Code of Conduct

This project adheres to a code of conduct that we expect all contributors to follow. Please read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before contributing.

## Getting Started

> **Important:** The `main` branch is the source of truth for stable code. All install scripts pull directly from `main` and build the binary. Keep `main` stable — use feature branches for development and ensure CI passes before merging.

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/itzcozi/coderaft.git
   cd coderaft
   ```
3. **Add the upstream repository**:
   ```bash
   git remote add upstream https://github.com/itzcozi/coderaft.git
   ```

## How to Contribute

### Types of Contributions

We welcome several types of contributions:

- 🐛 **Bug fixes** - Fix issues and improve reliability
- ✨ **New features** - Add functionality following our roadmap
- 📚 **Documentation improvements** - Keep docs accurate and helpful
- 🧪 **Tests** - Expand our comprehensive test suite
- 🔧 **Code refactoring** - Improve code quality and maintainability
- 🎨 **UI/UX improvements** - Enhance user experience
- ⚡ **Performance optimizations** - Make coderaft faster and more efficient
- 🛡️ **Security enhancements** - Improve security posture

### Good First Issues

Look for issues labeled with:
- `good first issue` - Perfect for newcomers
- `help wanted` - We'd love community help
- `documentation` - Improve our docs

## Development Setup

### Prerequisites

**Cross-Platform Support**: coderaft works on **Linux**, **macOS**, and **Windows** wherever Docker is available.

**Required Software**:
- Go 1.21 or later
- Docker (Docker Desktop on macOS/Windows)
- Git

### Local Setup

1. **Install dependencies**:
   ```bash
   make deps
   ```

2. **Build the project**:
   ```bash
   make build
   ```

3. **Run tests**:
   ```bash
   make test
   ```

4. **Run all quality checks**:
   ```bash
   make ci
   ```

### Development Workflow

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** following our coding standards

3. **Test your changes**:
   ```bash
   make ci
   ```

4. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```

5. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request** on GitHub

## Coding Standards

### Go Code Style

- Follow standard Go formatting with `gofmt`
- Use `golangci-lint` for code quality
- Write meaningful variable and function names
- Add comments for exported functions and types
- Keep functions small and focused (max 50 statements)
- Maintain cyclomatic complexity under 15

### Code Organization

```
cmd/           # Main applications
internal/      # Private application code
docs/          # Documentation
scripts/       # Build and utility scripts
.github/       # GitHub workflows and templates
```

### Commit Message Format

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(cli): add support for custom Docker registries
fix(config): resolve issue with JSON parsing
docs: update installation instructions
test(docker): add integration tests for container management
```

## Testing

### Overview

coderaft has a comprehensive test suite that includes:
- **Unit tests** for individual functions and components
- **Integration tests** for CLI commands and end-to-end functionality
- **Security tests** and vulnerability scanning
- **Performance tests** with race condition detection

### Test Structure

```
test/
├── integration/           # End-to-end CLI tests
│   └── cli_test.go
internal/
├── commands/
│   ├── version_test.go    # Version command tests
│   └── root_test.go       # Project validation tests
├── config/
│   ├── config_test.go     # Config struct tests
│   └── config_manager_test.go  # File operations tests
├── docker/
│   └── client_test.go     # Docker client tests
└── testutil/
    ├── testutil.go        # Test helpers and utilities
    └── testutil_test.go   # Tests for test utilities
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests with race detection (Linux)
go test -race ./...

# Run specific test packages
go test ./internal/commands -v
go test ./internal/config -v
go test ./test/integration -v

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Writing Tests

#### Unit Tests
- Write unit tests for all new functions
- Include both success and failure cases
- Use table-driven tests when appropriate
- Mock external dependencies (Docker, filesystem)
- Aim for meaningful test coverage (not just percentage)

#### Integration Tests
- Test CLI commands end-to-end
- Verify error messages and exit codes
- Handle OS-specific behavior gracefully
- Test argument validation and help text

#### Test Utilities
Use the `internal/testutil` package for common operations:

```go
import "coderaft/internal/testutil"

func TestMyFunction(t *testing.T) {
    // Create test data
    config := testutil.CreateTestConfig()
    project := testutil.CreateTestProject("my-project")

    // Use test assertions
    testutil.AssertNoError(t, err)
    testutil.AssertEqual(t, expected, actual)
    testutil.AssertNotNil(t, result)
}
```

### Test Guidelines

- **Fast**: Unit tests should run quickly (< 100ms each)
- **Isolated**: Tests shouldn't depend on external services
- **Deterministic**: Tests should produce consistent results
- **Clear**: Test names should describe what they verify
- **Comprehensive**: Cover happy paths, edge cases, and error conditions

### Test Structure

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    "test-project",
            expected: "test-project",
            wantErr:  false,
        },
        {
            name:    "invalid input",
            input:   "invalid@project",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := FunctionName(tt.input)

            if tt.wantErr {
                testutil.AssertError(t, err, "")
                return
            }

            testutil.AssertNoError(t, err)
            testutil.AssertEqual(t, tt.expected, result)
        })
    }
}
```

## Continuous Integration

### GitHub Actions Workflows

The project uses GitHub Actions for automated testing and quality assurance:

#### 1. Main CI Pipeline (`.github/workflows/ci.yml`)
Runs on every push and pull request:
- **Multi-version testing** (Go 1.21, 1.22)
- **Comprehensive test suite** with race detection
- **Cross-platform builds** (Linux, macOS, Windows)
- **Code linting** with golangci-lint
- **Security scanning** with gosec and govulncheck
- **Coverage reporting** with Codecov integration

#### 2. Linux Tests (`.github/workflows/cross-platform-tests.yml`)
Ensures stability on supported Linux runners:
- **Ubuntu** testing
- **Daily scheduled runs** for regression detection
- **Binary execution verification**

#### 3. Coverage Reporting (`.github/workflows/coverage.yml`)
Detailed test coverage analysis:
- **Atomic coverage mode** for accuracy
- **HTML and text reports** generation
- **Coverage trend tracking**
- **PR coverage comments**

#### 4. Test Status Summary (`.github/workflows/test-status.yml`)
Rich reporting in GitHub Actions:
- **Test result summaries**
- **Coverage percentage display**
- **Markdown-formatted reports**

### Quality Gates

All pull requests must pass:
- ✅ **All tests** (unit + integration)
- ✅ **Linting checks** (golangci-lint)
- ✅ **Security scans** (gosec, govulncheck)
- ✅ **Linux builds**
- ✅ **Code coverage** maintained or improved

### Local CI Simulation

Run the same checks locally before pushing:

```bash
# Full quality check suite
make quality

# Individual checks
make test          # Run all tests
make test-coverage # Generate coverage
make lint          # Code linting
make security      # Security scanning
make fmt           # Code formatting
```

### Automated Checks

The CI system automatically:
- **Runs tests** on multiple Go versions
- **Builds binaries** for all target platforms
- **Scans for vulnerabilities** in dependencies
- **Checks code formatting** and style
- **Validates documentation** changes
- **Generates coverage reports**
- **Creates build artifacts**

### Working with CI

#### When CI Fails:
1. **Check the logs** in GitHub Actions tab
2. **Run tests locally** to reproduce the issue
3. **Fix the issue** and push again
4. **Don't ignore CI failures** - they indicate real problems

#### Tips for Success:
- **Run tests locally** before pushing
- **Keep commits atomic** and focused
- **Write descriptive commit messages**
- **Update tests** when changing functionality
- **Check coverage** to ensure new code is tested

### Debugging CI Failures

Common CI issues and solutions:

#### Test Failures
```bash
# Reproduce locally
go test -v ./internal/package_name

# Check for race conditions (Linux)
go test -race ./...

# Run integration tests
go test -v ./test/integration
```

#### Linting Failures
```bash
# Run linter locally
make lint

# Auto-fix formatting issues
make fmt

# Check specific files
golangci-lint run ./internal/commands/
```

#### Coverage Issues
```bash
# Generate local coverage report
make test-coverage

# View coverage by function
go tool cover -func=coverage.out

# Open HTML coverage report
open coverage.html
```

#### Platform-Specific Issues

coderaft is designed to work on **Linux**, **macOS**, and **Windows** wherever Docker is available.

**Platform Notes**:
- **Linux**: Supports Debian, Ubuntu, Fedora, Arch, Alpine, and more
- **macOS**: Requires Docker Desktop for Mac
- **Windows**: Requires Docker Desktop for Windows

## Submitting Changes

### Pull Request Process

1. **Update documentation** if needed
2. **Add tests** for new functionality (unit and integration tests)
3. **Ensure all CI checks pass**:
   - ✅ Tests pass on all platforms
   - ✅ Linting checks pass
   - ✅ Security scans clear
   - ✅ Builds succeed for all targets
   - ✅ Coverage maintained or improved
4. **Update CHANGELOG.md** if applicable
5. **Request review** from maintainers

### Pre-submission Checklist

Before submitting your PR, run these commands locally:

```bash
# Run full test suite
make test

# Check code quality
make lint

# Verify security
make security

# Ensure proper formatting
make fmt

# Generate coverage report
make test-coverage
```

### Pull Request Guidelines

- **Title**: Use a descriptive title following conventional commits
- **Description**:
  - Explain what changes you made and why
  - Reference any related issues
  - Include screenshots for UI changes
  - List any breaking changes

### Pull Request Template

Your PR description should include:

```markdown
## Description
Brief description of changes and motivation

## Type of Change
- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that causes existing functionality to change)
- [ ] Documentation update
- [ ] Test improvement
- [ ] Refactoring (no functional changes)

## Testing
- [ ] Unit tests pass locally (`go test ./internal/...`)
- [ ] Integration tests pass locally (`go test ./test/...`)
- [ ] Added unit tests for new functionality
- [ ] Added integration tests if applicable
- [ ] All CI checks pass (tests, linting, security)

- [ ] Coverage maintained or improved

## Code Quality
- [ ] Code follows Go formatting (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] Security scans pass (`make security`)
- [ ] No race conditions (`go test -race ./...`)

## Documentation
- [ ] Updated relevant documentation
- [ ] Added code comments for complex logic
- [ ] Updated CHANGELOG.md if applicable
- [ ] README updated if needed

## Checklist
- [ ] Self-review completed
- [ ] No breaking changes (or clearly documented)
- [ ] Related issues referenced
- [ ] Screenshots included for UI changes
```

## Reporting Issues

### Bug Reports

When reporting bugs, please include:

1. **Clear title** describing the issue
2. **Environment details**:
   - OS and version
   - Go version
   - Docker version
   - coderaft version
3. **Steps to reproduce**
4. **Expected behavior**
5. **Actual behavior**
6. **Error messages or logs**
7. **Additional context**

## Feature Requests

We welcome feature requests! Please:

1. **Check existing issues** to avoid duplicates
2. **Describe the problem** you're trying to solve
3. **Propose a solution** if you have one
4. **Consider the scope** - start small and iterate
5. **Be open to discussion** about implementation

## Documentation

### Documentation Types

- **API documentation**: Inline code comments
- **User guides**: Markdown files in `docs/`
- **README updates**: For setup and usage
- **Changelog**: Track notable changes

### Documentation Standards

- Write clear, concise prose
- Include code examples
- Test all examples
- Keep documentation up-to-date with code changes (IMPORTANT)

## Community

### Communication Channels

- **GitHub Issues**: Bug reports, feature requests
- **GitHub Discussions**: General questions, ideas
- **Pull Requests**: Code review and discussion

### Getting Help

- Check existing documentation
- Search GitHub issues
- Create a new issue with the `question` label
- Join community discussions

## License

By contributing to coderaft, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to coderaft! 🚀
