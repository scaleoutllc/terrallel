# terrallel
> run terraform in parallel across dependent workspaces

# What is this?
Terrallel is used to trace dependencies in an operator defined `Infrafile` of
terraform workspaces. Terrallel will run any terraform command you supply in
every workspace under a defined target with maximum parallelism. Terrallel is
primarily designed for rapid creation and destruction of environments used for
testing.

## How does it work?
First, you define a YAML manifest describing workspaces. There are four
possible keys.

`targets`: top level key. define named logical workspaces within.

Within targets, the following keys are allowed:

`workspaces`: list of paths to workspaces that do not depend on eachother.

`group`: list of target names that do not depend on eachother.

`next`: nest additional dependent `group` and `workspaces` entries under this.

There is one rule: `workspaces` and `group` cannot be sibling to eachother. If
you require both in a target, they must be separated by a `next` key to nest
them. This design decision enforces a configuration format where the order of
operation is explicit. If `workspace` and `group` were allowed at the same
level of nesting, terrallel would have to make an implicit assumption about
which takes precedence to properly produce the dependency graph.

## Example
Here is a sample Infrafile from the `examples` directory. Examine that for
more context.

```yaml
terrallel:
  # This is a path relative to the Infrafile where all terraform workspaces
  # are located.
  basedir: environments
  # Targets can be defined in multiple files and imported only from the main
  # manifest file (Infrafile). If the same target is defined in multiple files
  # terrallel will error, no merging logic is supported.
  import:
  - terrallel/*.yml

# Targets can be defined in the main Infrafile. 
targets:
  dev:
    group:
    - dev-aws-networks
    - dev-gcp-networks
    next:
      workspaces:
      - dev/multi-cloud/networks
      next:
        group:
        - dev-gcp-clusters
        - dev-aws-clusters
        next:
          workspaces:
          - dev/multi-cloud/clusters
```

## Usage
```bash
terrallel dev -- init
terrallel dev --dry-run -- apply -auto-approve
terrallel dev -- apply -auto-approve
terrralel dev -- destroy -auto-approve
```