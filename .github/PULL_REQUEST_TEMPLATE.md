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
Labelling is a MAINTAINER step — contributors from a fork cannot set labels, and
you are not expected to. Just tick the box below and a maintainer will apply the
matching label before merge. If none is applied, a patch release is created.
-->

Which version bump should this get? (A maintainer applies the label.)

- [ ] **Major** (`release:major`) - Breaking change: removed flags/commands, changed config format, breaking behaviour
- [ ] **Minor** (`release:minor`) - New feature or enhancement, backwards compatible
- [ ] **Patch** (`release:patch`) - Bug fix, documentation, or small improvement
- [ ] **None** (`release:skip`) - Internal only, no user impact

## Testing

<!-- Describe the tests you ran to verify your changes -->

- [ ] Unit tests pass (`make test-unit`)
- [ ] Integration tests pass (if applicable)
- [ ] Manual testing completed
- [ ] Web UI tests pass (if applicable: `cd apps/web && bun run test:run`)

## Checklist

- [ ] My code follows the project's style guidelines
- [ ] I have performed a self-review of my code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have updated the documentation accordingly
- [ ] My changes generate no new warnings or errors
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New and existing unit tests pass locally with my changes
- [ ] I have indicated the release type above
- [ ] This PR does not contain credentials, real backup data, or `.env` / `bared.yml` / `*.db` files

<!--
Security: if this PR fixes a vulnerability that has not been publicly reported,
stop and report it privately first — see SECURITY.md. Do not describe the
vulnerability here.
-->

## Additional Context

<!-- Add any other context, screenshots, or information about the PR here -->
