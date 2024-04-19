# Developer's Guide

Welcome to the development reference for Camunda's C8 Multi Region! This document provides guidance on setting up a testing environment, running tests, and managing releases.

## Setting up Development Environment

TODO: this part is not yet documented outside of the official documentation: https://docs.camunda.io/docs/next/self-managed/setup/deploy/amazon/amazon-eks/dual-region/

## Releasing a New Version

We follow Semantic Versioning (SemVer) guidelines for versioning. Follow these steps to release a new version:

1. **Commit History:**
   - Maintain a clear commit history with explicit messages detailing additions and deletions.

2. **Versioning:**
   - Determine the appropriate version number based on the changes made since the last release.
   - Follow the format `MAJOR.MINOR.PATCH` as per Semantic Versioning guidelines.

3. **GitHub Releases:**
   - Publish the new version on GitHub Releases.
   - Tag the release with the version number and include release notes summarizing changes.

## Adding new GH actions

Please pin GitHub action, if you need you can use [pin-github-action](https://github.com/mheap/pin-github-action) cli tool.

---

By following these guidelines, we ensure smooth development iterations, robust testing practices, and clear version management for the Terraform EKS module. Happy coding!