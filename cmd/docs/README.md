
# ![HAProxy](../../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## Generator tool for markdown documentation

to run it use `make doc`

There are four parts of doc.yaml file:

- `active_version` - same as last stable version so we can generate dev and deprecate warnings
- `image_arguments` - data used for generating [controller.md](../controller.md)
  - example:

  ```yaml
  - argument: --some-argument
    description: descritpion about what is this argument
    values:
      - values that argument can have
    default: default value that argument have (if any)
    version_min: minimal version that have this feature (SEMVER, only MAJOR.MINOR)
    version_max: last version that have this feature
    example: |-
      args:
        - --some-argument=some value
  ```

- `groups` - data used for generating [README.md](../README.md), can be omitted if no additional info is needed
  - example:

  ```yaml
  tls-secret:
    header: |
      multiline string for entering additional data that is related to group of annotations
      can be written in markdown format
    footer: |
      multiline string for entering additional data that is related to group of annotations
      can be written in markdown format
  ```

- `annotations` - data used for generating [README.md](../README.md)
  - example:

  ```yaml
  - title: annotation-name
    type: type of data
    group: what group does it belong to (to keep related annotations together)
    dependencies: does it have any dependency (other annotation)
    default: default value
    description:
    - list of description lines for annotation
    tip:
    - extra info
    values:
    - list of values (can be descriptive) that annotation can have
    applies_to:
    - configmap
    - ingress
    - service
    version_min: minimal version that have this feature (SEMVER, only MAJOR.MINOR)
    version_max: last version that have this feature
    example:
    - list of examples (usually one)
    example_configmap: |-
      example that overrides global one
    example_ingress: |-
      example that overrides global one
    example_service: |-
      example that overrides global one
  ```
