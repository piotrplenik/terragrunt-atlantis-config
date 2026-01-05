# Atlantis Config for Terragrunt Projects.

## What is this?

[Atlantis](https://www.runatlantis.io) is a powerful tool for Terraform pull request automation that enables teams to
collaborate on infrastructure changes through pull requests.

[Terragrunt](https://terragrunt.gruntwork.io) is a thin wrapper around Terraform that helps manage large, multi-module
configurations. It keeps configurations DRY, manages remote state, and natively supports inter-module dependencies.

### The Problem

In large Terragrunt repositories—especially monorepos—manually creating and maintaining atlantis.yaml becomes tedious
and error-prone due to the sheer number of modules and dependency relationships.


### The Solution

`terragrunt-atlantis-config` automatically generates an `atlantis.yaml` for Terragrunt projects by:
 - Discovering all terragrunt.hcl, terragrunt.stack.hcl, and terragrunt.hcl.json files
 - Parsing dependency and configuration blocks
 - Building a dependency DAG
 - Producing a complete, dependency-aware atlantis.yaml

This makes Atlantis usable at scale for complex Terragrunt setups without manual configuration overhead.

### Key Benefits

 - **Automatic Dependencies**: Plans dependent modules automatically using Atlantis [project dependencies](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#project-dependencies)
 - **Workflow Integration**: Works seamlessly with [custom workflows](https://www.runatlantis.io/docs/custom-workflows.html) and [autoplanning](https://www.runatlantis.io/docs/autoplanning.html)
- **Parallel Execution**: Enables [parallel plan/apply](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#parallel-plan-and-apply) with isolated workspaces
- **No Manual Upkeep**: Configuration updates automatically as you modify your Terragrunt modules

## Prerequisites

Before using this tool, ensure you have:

1. **Atlantis >v0.26.0**: A running Atlantis instance (see [Atlantis Installation Guide](https://www.runatlantis.io/docs/installation-guide.html)) -
2. **Terragrunt >v0.97.0**: A repository with Terragrunt configurations

## Integrate into your Atlantis Server

The recommended way to use this tool is to install it onto your Atlantis server, and then use a [Pre-Workflow Hook](https://www.runatlantis.io/docs/pre-workflow-hooks.html) to run it after every repository clone. This way, Atlantis can automatically generate the configuration and determine what modules should be planned/applied for any change to your repository.

Pre-workflow hooks run before Atlantis workflows execute, making them ideal for dynamic configuration generation. They're defined in the [server-side repo config](https://www.runatlantis.io/docs/server-side-repo-config.html), which is separate from the repo-level `atlantis.yaml` that this tool generates.

### Step 1: Configure the Pre-Workflow Hook

To get started, add a `pre_workflow_hooks` field to your `repos` section of your [custom workflow](https://www.runatlantis.io/docs/custom-workflows.html#terragrunt#do-i-need-a-server-side-repo-config-file):

```yaml
---
repos:
  - id: /.*/
    workflow: terragrunt
    pre_workflow_hooks:
      - run: terragrunt-atlantis-config generate --output atlantis.yaml --autoplan --parallel --create-workspace --automerge
workflows:
  terragrunt:
    plan:
      steps:
        - env:
            name: TF_IN_AUTOMATION
            value: 'true'
        - run: find . -name '.terragrunt-cache' | xargs rm -rf
        # - run: terragrunt init -reconfigure
        - run:
            command: terragrunt plan -input=false -out=$PLANFILE
            output: hide
        - run: terragrunt --log-custom-format "%msg" show $PLANFILE
    apply:
      steps:
        - run: terragrunt apply --log-custom-format "%msg" $PLANFILE
```

**Common flags explained:**
- `--autoplan`: Enables [autoplanning](https://www.runatlantis.io/docs/autoplanning.html) - Atlantis automatically runs `plan` when PRs are opened/updated
- `--parallel`: Enables parallel [plan and apply](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#parallel-plan-and-apply) operations
- `--create-workspace`: Creates separate [workspaces](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#workspace) for each project to enable parallelism
- `--output`: Specifies where to write the generated `atlantis.yaml` file

Learn more about available flags in the [All Flags](#all-flags) section below.

### Step 2: Install terragrunt-atlantis-config on Your Server

Then, make sure `terragrunt-atlantis-config` is present on your Atlantis server.

```bash
#!/bin/sh
set -eoux pipefail
# Terragrunt
TG_VERSION="0.96.1"
TG_SHA256_SUM="513eff2f87e2f5ec84369cc0f9d6c6766b43ca765fec4a3ac3598b933dc3218f"
TG_FILE="${INIT_SHARED_DIR}/terragrunt"
wget https://github.com/gruntwork-io/terragrunt/releases/download/v${TG_VERSION}/terragrunt_linux_amd64 -O "${TG_FILE}"
echo "${TG_SHA256_SUM} ${TG_FILE}" | sha256sum -c
chmod 755 "${TG_FILE}"
terragrunt -v

# OpenTofu
TF_VERSION="1.11.2"
TF_FILE="${INIT_SHARED_DIR}/tofu"
TF_SHA256_SUM="1bfb425c940098952df7a74e2f67dd318614bbea2767a25de94e46ca5d5b85ec"
wget https://github.com/opentofu/opentofu/releases/download/v${TF_VERSION}/tofu_${TF_VERSION}_linux_amd64.zip -O "tofu.zip"
echo "${TF_SHA256_SUM} tofu.zip" | sha256sum -c
unzip tofu.zip
mv tofu ${INIT_SHARED_DIR}
chmod 755 "${TF_FILE}"
tofu -v

# terragrunt-atlantis-config
TAC_VERSION="2.23.0"
TAC_SHA256_SUM="a6e77d2bcb554e470fd3396989f9c17061a7195571a657696885b5c73ac09d4f"
TAC_FILE="${INIT_SHARED_DIR}/terragrunt-atlantis-config"
wget "https://github.com/piotrplenik/terragrunt-atlantis-config/releases/download/v${TAC_VERSION}/terragrunt-atlantis-config_${TAC_VERSION}_linux_amd64"
echo "${TAC_SHA256_SUM} terragrunt-atlantis-config_${TAC_VERSION}_linux_amd64" | sha256sum -c
cp -fv "terragrunt-atlantis-config_${TAC_VERSION}_linux_amd64" "${TAC_FILE}"
chmod 755 "${TAC_FILE}"
terragrunt-atlantis-config version
```

More info [Atlantis Deployment](https://www.runatlantis.io/docs/deployment.html).

## How It Works

On each pull request:

1. **Clone**: Atlantis clones the Terragrunt repository.
2. **Generate Config**: pre-workflow hook runs `terragrunt-atlantis-config generate` to:
   - Discover Terragrunt modules and dependencies
   - Build a dependency graph
   - Generates an `atlantis.yaml`
3. **Plan**: Atlantis plans affected projects using [autoplanning](https://www.runatlantis.io/docs/autoplanning.html)
4. **Apply**: Apply `atlantis apply`, executes in dependency order.

This ensures dependencies stay current, downstream modules are planned automatically, and applies are executed safely in the correct order.

## Extra dependencies

### Configuration

Add an `extra_atlantis_dependencies` field to the `locals` block in your `terragrunt.hcl`:

```hcl
locals {
  extra_atlantis_dependencies = [
    "some_extra_dep",
    find_in_parent_folders(".gitignore")
  ]
}
```

This will be reflected in the generated `atlantis.yaml` in the [`when_modified`](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#when_modified) section:

```yaml
- autoplan:
    enabled: false
    when_modified:
      - "*.hcl"
      - "*.tf*"
      - some_extra_dep
      - ../../.gitignore
  dir: example-setup/extra_dependency
```

## All Flags

One way to customize the behavior of this module is through CLI flag values passed in at runtime. These settings will apply to all modules.

Many of these flags directly correspond to [Atlantis repo-level configuration options](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html). Understanding the Atlantis configuration model will help you use these flags effectively.

| Flag Name                    | Description                                                                                                                                                                     | Default Value     |
|------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------|
| `--autoplan`                 | The default value for autoplan settings. Can be overriden by locals.                                                                                                            | false             |
| `--automerge`                | Enables the automerge setting for a repo.                                                                                                                                       | false             |
| `--cascade-dependencies`     | When true, dependencies will cascade, meaning that a module will be declared to depend not only on its dependencies, but all dependencies of its dependencies all the way down. | true              |
| `--ignore-parent-terragrunt` | Ignore parent Terragrunt configs (those which don't reference a terraform module).<br>In most cases, this should be set to `true`                                               | true              |
| `--parallel`                 | Enables `plan`s and `apply`s to happen in parallel. Will typically be used with `--create-workspace`                                                                            | true              |
| `--create-workspace`         | Use different auto-generated workspace for each project. Default is use default workspace for everything                                                                        | false             |
| `--create-project-name`      | Add different auto-generated name for each project                                                                                                                              | false             |
| `--preserve-workflows`       | Preserves workflows from old output files. Useful if you want to define your workflow definitions on the client side                                                            | true              |
| `--preserve-projects`        | Preserves projects from old output files. Useful for incremental builds using `--filter`                                                                                        | false             |
| `--workflow`                 | Name of the workflow to be customized in the atlantis server. If empty, will be left out of output                                                                              | ""                |
| `--apply-requirements`       | Requirements that must be satisfied before `atlantis apply` can be run. Currently the only supported requirements are `approved` and `mergeable`. Can be overridden by locals   | []                |
| `--output`                   | Path of the file where configuration will be generated. Typically, you want a file named "atlantis.yaml". Default is to write to `stdout`.                                      | ""                |
| `--root`                     | Path to the root directory of the git repo you want to build config for.                                                                                                        | current directory |
| `--terraform-version`        | Default terraform version to specify for all modules. Can be overriden by locals                                                                                                | ""                |
| `--ignore-dependency-blocks` | When true, dependencies found in `dependency` and `dependencies` blocks will be ignored                                                                                         | false             |
| `--filter`                   | Path or glob expression to the directory you want scope down the config for. Default is all files in root                                                                       | ""                |
| `--num-executors`            | Number of executors used for parallel generation of projects. Default is 15                                                                                                     | 15                |
| `--execution-order-groups`   | Computes execution_order_group for projects                                                                                                                                     | false             |
| `--depends-on`               | Computes depends_on for projects. Project names are required.                                                                                                                   | false             |

**Key flags for Atlantis integration:**
- Use `--apply-requirements` to enforce [apply requirements](https://www.runatlantis.io/docs/apply-requirements.html) like PR approval before applying changes
- Use `--workflow` to specify a [custom workflow](https://www.runatlantis.io/docs/custom-workflows.html) defined in your server-side config
- Combine `--parallel` and `--create-workspace` to enable [parallel operations](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#parallel-plan-and-apply)

## Project generation

These flags offer additional options to generate Atlantis projects based on HCL configuration files in the terragrunt hierarchy. This, for example, enables Atlantis to use `terragrunt run-all` workflows on staging environment or product levels in a terragrunt hierarchy. Mostly useful in large terragrunt projects containing lots of interdependent child modules. Atlantis `locals` can be used in the defined project marker files.

| Flag Name                    | Description                                                                                                                                                                     | Default Value     | Type |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------- |----- |
| `--project-hcl-files`        | Comma-separated names of arbitrary hcl files in the terragrunt hierarchy to create Atlantis projects for.<br>Disables the `--filter` flag  | ""      |  list(string) |
| `--use-project-markers`      | If enabled, project hcl files must include `locals { atlantis_project = true }` for project creation.  | false      |  bool |
| `--create-hcl-project-childs`        | Creates Atlantis projects for terragrunt child modules below the directories containing the HCL files defined in --project-hcl-files  | false       | bool |
| `--create-hcl-project-external-childs`    | Creates Atlantis projects for terragrunt child modules outside the directories containing the HCL files defined in --project-hcl-files  | true          | bool |

## All Locals

Another way to customize the output is to use `locals` values in your terragrunt modules. These can be set in either the parent or child terragrunt modules, and the settings will only affect the current module (or all child modules for parent locals).

| Locals Name                   | Description                                                                                                                                                    | type         |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| `atlantis_workflow`           | The custom atlantis workflow name to use for a module                                                                                                          | string       |
| `atlantis_apply_requirements` | The custom `apply_requirements` array to use for a module                                                                                                      | list(string) |
| `atlantis_terraform_version`  | Allows overriding the `--terraform-version` flag for a single module                                                                                           | string       |
| `atlantis_autoplan`           | Allows overriding the `--autoplan` flag for a single module                                                                                                    | bool         |
| `atlantis_skip`               | If true on a child module, that module will not appear in the output.<br>If true on a parent module, none of that parent's children will appear in the output. | bool         |
| `extra_atlantis_dependencies` | See [Extra dependencies](https://github.com/piotrplenik/terragrunt-atlantis-config#extra-dependencies)                                                        | list(string) |
| `atlantis_project`            | Create Atlantis project for a project hcl file. Only functional with `--project-hcl-files` and `--use-project-markers` | bool         |

## Separate workspace for parallel plan and apply

Atlantis added support for running plan and apply in parallel in [v0.13.0](https://github.com/runatlantis/atlantis/releases/tag/v0.13.0). This feature allows multiple Terraform operations to run simultaneously, significantly speeding up large infrastructure changes.

To use this feature, projects must be separated into different [workspaces](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#workspace). The `--create-workspace` flag enables this by concatenating the project path as the workspace name.

**Example:** Project `${git_root}/stage/app/terragrunt.hcl` will have `stage_app` as its workspace name.

This flag should be used together with `--parallel` to enable [parallel plan and apply](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#parallel-plan-and-apply):

```bash
terragrunt-atlantis-config generate --output atlantis.yaml --parallel --create-workspace
```

**Important considerations:**
- Enabling this feature may consume more resources (CPU, memory, network, disk) as each workspace will be cloned separately by Atlantis
- When running commands on specific directories, include the workspace: `atlantis plan/apply -d ${git_root}/stage/app -w stage_app`
- Alternatively, if you enable `--create-project-name`, you can use project-based commands: `atlantis plan/apply -p stage_app`

Learn more about [Atlantis workspaces](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#workspace) and how they relate to [Terraform workspaces](https://www.runatlantis.io/docs/terraform-workspaces.html).

## Rules for merging config

Each terragrunt module can have locals, but can also have zero to many `include` blocks that can specify parent terragrunt files that can also have locals.

In most cases (for string/boolean locals), the primary terragrunt module has the highest precedence, followed by the locals in the lowest appearing `include` block, etc. all the way until the lowest precedence at the locals in the first `include` block to appear.

However, there is one exception where the values are merged, which is the `atlantis_extra_dependencies` local. For this local, all values are appended to one another. This way, you can have `include` files declare their own dependencies.

## Local Installation and Usage

You can install this tool locally to preview the Atlantis configuration it will generate for your repository. This is useful for testing and debugging before deploying to production.

**Note:** In production, it's recommended to [install this tool directly on your Atlantis server](#integrate-into-your-atlantis-server) and use pre-workflow hooks.

### Installation

Recommended: Install any version via go install:

```bash
go install github.com/piotrplenik/terragrunt-atlantis-config@v1.17.9
```

This module officially supports golang version v1.21, tested on Github with each build. 
This module also officially supports both Windows and Nix-based file formats, tested on Github with each build. 

Usage Examples (see below sections for all options):

```bash
# From the root of your repo
terragrunt-atlantis-config generate

# or from anywhere
terragrunt-atlantis-config generate --root /some/path/to/your/repo/root

# output to a file
terragrunt-atlantis-config generate --autoplan --output ./atlantis.yaml
```

Finally, check the log output (or your output file) for the YAML.
