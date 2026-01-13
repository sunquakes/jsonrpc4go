# Contributing to jsonrpc4go

Thank you for considering contributing to jsonrpc4go! We welcome contributions from everyone, and we appreciate your efforts to make this project better.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

### Reporting Bugs
- Ensure the bug was not already reported by searching existing issues
- Use a clear and descriptive title
- Include as much relevant information as possible
- Provide a minimal reproduction case

### Suggesting Enhancements
- Use a clear and descriptive title
- Provide a step-by-step description of the suggested enhancement
- Explain why this enhancement would be useful

### Pull Requests
- Fork the repository and create your branch from `main`
- Follow the existing code style
- Write clear, descriptive commit messages
- Add tests for new functionality
- Ensure all tests pass before submitting
- Include documentation for new features
- Submit your pull request to the `main` branch

## Development Setup

1. Clone your fork of the repository
2. Navigate to the project directory
3. Run `go mod download` to install dependencies
4. Make your changes
5. Run tests: `go test ./...`
6. Format your code: `go fmt ./...`

## Code Review Process

All submissions require review by project maintainers. When you submit a pull request:

1. Maintainers will review your code for correctness, style, and performance
2. Address any feedback provided during the review
3. Once approved, your changes will be merged

## Security Considerations

When contributing code, please consider:
- Input validation and sanitization
- Memory safety
- Potential denial-of-service vectors
- Proper error handling

## Testing

All code changes should include appropriate tests. We use Go's standard testing framework. When adding new functionality:
- Add unit tests to cover the new code
- Ensure existing tests continue to pass
- Consider edge cases and error conditions

## Documentation

- Update README.md if your changes affect usage
- Add comments to exported functions/types
- Keep documentation clear and up-to-date