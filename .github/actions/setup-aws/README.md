# Setup AWS and tooling

## Description

A composite action to deduplicate the code.
It's setting up the AWS CLI, pull the required secrets, and minium tooling for Go and Terraform.


## Inputs

| name | description | required | default |
| --- | --- | --- | --- |
| `secrets` | <p>JSON wrapped secrets for easier secret passing (one can use <code>{{ toJSON(secrets) }}</code>)</p> | `true` | `""` |
| `region` | <p>Region to use for the AWS Profile</p> | `false` | `eu-west-2` |


## Runs

This action is a `composite` action.

## Usage

```yaml
- uses: camunda/c8-multi-region/.github/actions/setup-aws@main
  with:
    secrets:
    # JSON wrapped secrets for easier secret passing (one can use `{{ toJSON(secrets) }}`)
    #
    # Required: true
    # Default: ""

    region:
    # Region to use for the AWS Profile
    #
    # Required: false
    # Default: eu-west-2
```
