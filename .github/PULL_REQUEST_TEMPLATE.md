# Pull Request

## Description

<!-- Provide a brief description of the changes in this PR -->

## Type of Change

<!-- Check all that apply -->

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Code refactoring
- [ ] Performance improvement
- [ ] Test additions/updates

## Release Type

<!--
Add the appropriate release label to this PR:
- release:major - Breaking changes (x.0.0)
- release:minor - New features (0.x.0)
- release:patch - Bug fixes, docs, small improvements (0.0.x)
- release:skip - No release needed (internal changes, WIP)

If no label is added, the default will be a patch release.
-->

**Required:** Add one of the following labels to this PR:

- `release:major` - For breaking changes or major new features
- `release:minor` - For new features and enhancements
- `release:patch` - For bug fixes, documentation, and small improvements
- `release:skip` - To skip automatic release (internal changes only)

## Testing

<!-- Describe the tests you ran to verify your changes -->

- [ ] Unit tests pass (`make test-unit`)
- [ ] Integration tests pass (if applicable)
- [ ] Manual testing completed
- [ ] Web UI tests pass (if applicable: `cd apps/web && npm run test:run`)

## Checklist

- [ ] My code follows the project's style guidelines
- [ ] I have performed a self-review of my code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have updated the documentation accordingly
- [ ] My changes generate no new warnings or errors
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New and existing unit tests pass locally with my changes
- [ ] I have added the appropriate release label (see Release Type above)

## Additional Context

<!-- Add any other context, screenshots, or information about the PR here -->
