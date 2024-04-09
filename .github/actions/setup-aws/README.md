# Setup AWS and tooling

## Intro

A composite action to deduplicate the code.

It's setting up the AWS CLI, pull the required secrets, and minium tooling for Go and Terraform.

## Usage

### Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| secrets | JSON wrapped secrets for easier secret passing | true |         |

## Example of using the action

```yaml
steps:
- uses: actions/checkout@v3
- name: Setup AWS and Tooling
    uses: ./.github/actions/setup-aws
    with:
        secrets: ${{ toJSON(secrets) }}
```
