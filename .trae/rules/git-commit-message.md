---
alwaysApply: true
scene: git_message
---

# Git Commit Message Rules (English Only)

## Language Requirement
- **ALL commit messages MUST be in English ONLY**
- **DO NOT use any other language (no Chinese, no other languages)**
- This applies to ALL parts: subject line, body, and footer
- This is a STRICT requirement - always generate English commit messages

## Format
- Commit messages must be in **English**
- Use imperative mood (e.g., "Add", "Fix", "Update", "Remove")
- Keep the first line under 50 characters (subject line)
- Separate subject from body with a blank line
- Wrap body lines at 72 characters
- Body bullet points MUST also be in English

## Structure
```
<type>(<scope>): <subject>

<body>

<footer>
```

## Allowed Types
- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, missing semicolons, etc.)
- **refactor**: Code refactoring (no functional/behavioral changes)
- **test**: Adding tests or fixing tests
- **chore**: Build/tooling changes (no source code changes)
- **perf**: Performance improvements
- **ci**: CI/CD configuration changes
- **revert**: Reverting a previous commit

## Examples

### Good Examples
```
feat(auth): add JWT token authentication
```

```
fix(api): resolve null pointer exception in user service

- Add null check before accessing user object
- Improve error handling in login flow
```

```
docs(readme): update installation instructions

- Add prerequisites section
- Include Docker installation guide
```

```
refactor(utils): simplify string processing logic

- Remove redundant helper functions
- Improve code readability
- Reduce cyclomatic complexity
```

### Bad Examples
- ❌ "fixed the thing" - Too vague, not imperative
- ❌ "I added a new feature" - Uses first person, not imperative
- ❌ "Bug fix for login page" - Missing type prefix
- ❌ "update dependencies and fix some bugs" - Mixes unrelated changes

## Best Practices
1. Keep messages clear and concise
2. Focus on **what** was changed, not **how**
3. Reference issue numbers when applicable (e.g., "fix: resolve #123")
4. Use consistent terminology
5. Capitalize the first letter of the subject
6. Do not end the subject with a period