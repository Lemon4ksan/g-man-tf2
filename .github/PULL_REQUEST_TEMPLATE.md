## Description

Provide a clear and concise summary of the changes made and the architectural rationale behind them.

## Related Issues

Link any related GitHub issues or discussions (e.g., Closes #123).

## Type of Change

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Code refactoring (O(1) optimization, structure cleanups)
- [ ] Documentation update (READMEs, package guides)

## Contribution Checklist

Please verify that your pull request meets all requirements:

- [ ] My code follows the code style of this project (`go fmt` / `go vet` clean).
- [ ] I have performed a self-review of my own code.
- [ ] I have commented my code, particularly in hard-to-understand areas or low-level binary packet parses.
- [ ] I have made corresponding changes to the documentation (READMEs and `CONTRIBUTING.md`).
- [ ] My changes generate no new warnings or race conditions when verified with `go test -race ./...`.
- [ ] I have added table-driven or unit tests that prove my fix is effective or that my feature works.
- [ ] All new and existing tests pass.
