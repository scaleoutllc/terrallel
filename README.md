# terrallel
> maximize parallel execution of terraform

# What is this?
Terrallel is used to trace dependencies in an operator defined `Infrafile` of
terraform workspaces. Terrallel will run any terraform command you supply in
in every workspace under a defined target with maximum parallelism. Terrallel
is primarily designed for rapid creation and destruction of environments used
for testing.

## How does it work?
You define a YAML manifest describing workspaces. There are only four possible
keys.

`targets`: top level key. define named logical workspaces within.

Within targets, the following keys are allowed:

`workspaces`: list of paths to workspaces that do not depend on eachother.

`group`: list of target names that do not depend on eachother.

`next`: nest additional dependent `group` and `workspaces` entries under this.

There is one simple rule: `workspaces` and `group` cannot be sibling to
eachother. If you require both in a target, they must be separated by a
`next` key to nest them. This design decision enforces a configuration
format where the order of operation is explicit. If `workspace` and `group`
were allowed at the same level of nesting, terrallel would have to make an
implicit assumption about which takes precedence to properly produce the
dependency graph.

## Example
Here is a sample Infrafile:

```yaml
config:
  basedir: environments

targets:
  numbers:
    group:
    - even
    - odd
    next:
      workspaces:
      - end-all

  odd:
    workspaces:
    - one
    - three
    - five
    next:
      workspaces:
      - end-odd
    
  even:
    workspaces:
    - two
    - four
    - six
    next:
      workspaces:
      - end-even
```