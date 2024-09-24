# Setup AWS

## Description

Step wrapper to setup AWS credentials and region.


## Inputs

| name | description | required | default |
| --- | --- | --- | --- |
| `secrets` | <p>JSON wrapped secrets for easier secret passing</p> | `true` | `""` |
| `region` | <p>AWS region to use</p> | `false` | `eu-west-2` |


## Runs

This action is a `composite` action.

## Usage

```yaml
- uses: camunda/c8-multi-region/.github/actions/setup-aws@main
  with:
    secrets:
    # JSON wrapped secrets for easier secret passing
    #
    # Required: true
    # Default: ""

    region:
    # AWS region to use
    #
    # Required: false
    # Default: eu-west-2
```
