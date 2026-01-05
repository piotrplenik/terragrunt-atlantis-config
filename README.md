# Atlantis Config for Terragrunt Projects.

## What is this?

[Atlantis](https://www.runatlantis.io) is a powerful tool for Terraform pull request automation that enables teams to collaborate on infrastructure changes through pull requests. It runs `terraform plan` and `terraform apply` directly from pull requests, providing visibility and control over infrastructure changes. Each repository can have a YAML configuration file ([`atlantis.yaml`](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html)) that defines Terraform module dependencies, workflows, and automation rules.

[Terragrunt](https://terragrunt.gruntwork.io) is a thin wrapper for Terraform that provides extra tools for keeping your configurations DRY, working with multiple Terraform modules, and managing remote state. Terragrunt has built-in support for defining dependencies between modules.

**The Challenge:** While Atlantis excels at automating Terraform workflows and Terragrunt excels at managing complex Terraform configurations, manually creating and maintaining an `atlantis.yaml` file for large Terragrunt projects with hundreds of interdependent modules is tedious and error-prone.

**The Solution:** `terragrunt-atlantis-config` automatically generates Atlantis configurations for Terragrunt projects by:

- Finding all `terragrunt.hcl`, `terragrunt.stack.hcl` and `terragrunt.hcl.json` files in a repository
- Evaluating their `dependency`, `dependencies`, `terraform`, `locals`, and other blocks to discover module relationships
- Building a Directed Acyclic Graph (DAG) of all dependencies
- Generating a complete [`atlantis.yaml`](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html) file that reflects the dependency graph

This automation is especially valuable for organizations using monorepos for their Terragrunt configurations, where manually maintaining dependency information across hundreds or thousands of modules would be impractical.

### Key Benefits

- **Automatic Dependency Detection**: Leverages Atlantis' [project dependencies](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#project-dependencies) feature to ensure dependent modules are planned when their dependencies change
- **Workflow Automation**: Integrates with Atlantis [custom workflows](https://www.runatlantis.io/docs/custom-workflows.html) and [autoplanning](https://www.runatlantis.io/docs/autoplanning.html)
- **Parallel Execution**: Supports Atlantis' [parallel plan/apply](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#parallel-plan-and-apply) using separate workspaces
- **Zero Manual Maintenance**: Configuration updates automatically as you modify your Terragrunt modules

## Prerequisites

Before using this tool, ensure you have:

1. **Atlantis Server**: A running Atlantis instance (see [Atlantis Installation Guide](https://www.runatlantis.io/docs/installation-guide.html))
2. **Terragrunt Project**: A repository with Terragrunt configurations
3. **Understanding of Atlantis Concepts**:
   - [Repo-level atlantis.yaml](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html) - The configuration file this tool generates
   - [Server-side repo config](https://www.runatlantis.io/docs/server-side-repo-config.html) - Where you'll configure pre-workflow hooks
   - [Workflows](https://www.runatlantis.io/docs/custom-workflows.html) - Custom sequences of commands Atlantis runs
   - [Autoplanning](https://www.runatlantis.io/docs/autoplanning.html) - Automatic terraform plan on PR changes

## Integrate into your Atlantis Server

The recommended way to use this tool is to install it onto your Atlantis server, and then use a [Pre-Workflow Hook](https://www.runatlantis.io/docs/pre-workflow-hooks.html) to run it after every repository clone. This way, Atlantis can automatically generate the configuration and determine what modules should be planned/applied for any change to your repository.

Pre-workflow hooks run before Atlantis workflows execute, making them ideal for dynamic configuration generation. They're defined in the [server-side repo config](https://www.runatlantis.io/docs/server-side-repo-config.html), which is separate from the repo-level `atlantis.yaml` that this tool generates.

### Step 1: Configure the Pre-Workflow Hook

To get started, add a `pre_workflow_hooks` field to your `repos` section of your [server-side repo config](https://www.runatlantis.io/docs/server-side-repo-config.html#do-i-need-a-server-side-repo-config-file):

```json
{
  "repos": [
    {
      "id": "<your_github_repo>",
      "workflow": "default",
      "pre_workflow_hooks": [
        {
          "run": "terragrunt-atlantis-config generate --output atlantis.yaml --autoplan --parallel --create-workspace"
        }
      ]
    }
  ]
}
```

**Common flags explained:**
- `--autoplan`: Enables [autoplanning](https://www.runatlantis.io/docs/autoplanning.html) - Atlantis automatically runs `plan` when PRs are opened/updated
- `--parallel`: Enables parallel [plan and apply](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#parallel-plan-and-apply) operations
- `--create-workspace`: Creates separate [workspaces](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#workspace) for each project to enable parallelism
- `--output`: Specifies where to write the generated `atlantis.yaml` file

Learn more about available flags in the [All Flags](#all-flags) section below.

### Step 2: Install terragrunt-atlantis-config on Your Server

Then, make sure `terragrunt-atlantis-config` is present on your Atlantis server. There are many different ways to configure a server, but this example in [Packer](https://www.packer.io/) should show the bash commands you'll need just about anywhere:

```hcl
variable "terragrunt_atlantis_config_version" {
  default = "2.23.0"
}

build {
  // ...
  provisioner "shell" {
    inline = [
      "wget https://github.com/piotrplenik/terragrunt-atlantis-config/releases/download/v${var.terragrunt_atlantis_config_version}/terragrunt-atlantis-config_${var.terragrunt_atlantis_config_version}_linux_amd64.tar.gz",
      "sudo tar xf terragrunt-atlantis-config_${var.terragrunt_atlantis_config_version}_linux_amd64.tar.gz",
      "sudo mv terragrunt-atlantis-config_${var.terragrunt_atlantis_config_version}_linux_amd64/terragrunt-atlantis-config_${var.terragrunt_atlantis_config_version}_linux_amd64 terragrunt-atlantis-config",
      "sudo install terragrunt-atlantis-config /usr/local/bin",
    ]
    inline_shebang = "/bin/bash -e"
  }
  // ...
}
```

**Alternative installation methods:**
- **Docker**: Include the binary in your Atlantis Docker image (see [Atlantis Docker documentation](https://www.runatlantis.io/docs/deployment.html#docker))
- **Kubernetes**: Add as an init container or include in your Atlantis pod image (see [Atlantis Kubernetes Guide](https://www.runatlantis.io/docs/deployment.html#kubernetes))
- **Binary releases**: Download from [GitHub Releases](https://github.com/piotrplenik/terragrunt-atlantis-config/releases)

Once configured, your developers will never need to worry about maintaining an `atlantis.yaml` file manually—it will be generated automatically on each Atlantis run.

## How It Works

When Atlantis receives a pull request:

1. **Clone**: Atlantis clones your repository (standard [Atlantis behavior](https://www.runatlantis.io/docs/how-atlantis-works.html))
2. **Pre-Workflow Hook**: The hook runs `terragrunt-atlantis-config generate`, which:
   - Scans your repository for all `terragrunt.hcl` files
   - Parses dependencies from `dependency` and `dependencies` blocks
   - Evaluates `locals` blocks for custom configurations
   - Builds a dependency graph
   - Generates an `atlantis.yaml` with proper [project configurations](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#projects)
3. **Planning**: Atlantis uses the generated config to determine which projects to plan (see [Autoplanning](https://www.runatlantis.io/docs/autoplanning.html))
4. **Apply**: When you comment `atlantis apply`, Atlantis respects the dependency order defined in the generated config

This workflow ensures that:
- Dependencies are always up-to-date
- Module changes trigger plans for dependent modules
- Apply operations respect dependency order
- You can leverage [apply requirements](https://www.runatlantis.io/docs/apply-requirements.html) for safety

## Extra dependencies

For basic cases, this tool automatically detects all module dependencies from your Terragrunt configuration. However, you may need additional dependencies for advanced scenarios:

**Common use cases:**
- You use Terragrunt's `read_terragrunt_config` function in your locals and want to depend on the read file
- Your Terragrunt module should trigger a plan when non-Terragrunt files change (e.g., Dockerfiles, Packer templates, scripts)
- You want to run _all_ modules when certain critical files change (e.g., major version bumps)
- You need custom file watching beyond the [default autoplan behavior](https://www.runatlantis.io/docs/autoplanning.html#customizing-when-modified)

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

### Merging Parent and Child Dependencies

If you specify `extra_atlantis_dependencies` in the parent Terragrunt module, they will be merged with the child dependencies using the following rules:

1. **Functions**: Any function in a parent will be evaluated from the child's directory. You can use `get_parent_terragrunt_dir()` and other [Terragrunt built-in functions](https://terragrunt.gruntwork.io/docs/reference/built-in-functions/) as you normally would
2. **Absolute paths**: Work as they would in a child module; the output path will be relative from the child module to the absolute path
3. **Relative paths**: Evaluated relative to the _child_ module. For paths relative to the parent module, use `"${get_parent_terragrunt_dir()}/foo.json"`

This merging behavior gives you flexibility to define common dependencies at the parent level while allowing child modules to add their own specific dependencies. Learn more about [Terragrunt configuration inheritance](https://terragrunt.gruntwork.io/docs/features/keep-your-terragrunt-architecture-dry/#dry-common-terragrunt-configuration).

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

## Further Reading

To get the most out of this tool and Atlantis, we recommend reviewing the following Atlantis documentation:

### Essential Atlantis Documentation

- **[How Atlantis Works](https://www.runatlantis.io/docs/how-atlantis-works.html)** - Understanding the Atlantis workflow and lifecycle
- **[Repo-Level atlantis.yaml](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html)** - Complete reference for the configuration file this tool generates
- **[Server-Side Repo Config](https://www.runatlantis.io/docs/server-side-repo-config.html)** - Where to configure pre-workflow hooks and global settings
- **[Custom Workflows](https://www.runatlantis.io/docs/custom-workflows.html)** - Customize the commands Atlantis runs for plan/apply
- **[Pre-Workflow Hooks](https://www.runatlantis.io/docs/pre-workflow-hooks.html)** - Run commands before workflows (where this tool runs)

### Advanced Topics

- **[Autoplanning](https://www.runatlantis.io/docs/autoplanning.html)** - Automatic planning on PR changes
- **[Apply Requirements](https://www.runatlantis.io/docs/apply-requirements.html)** - Require approvals, mergeable status, etc. before applying
- **[Parallel Plans and Applies](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#parallel-plan-and-apply)** - Run multiple operations simultaneously
- **[Project Dependencies](https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#project-dependencies)** - Define module dependencies (automatically generated by this tool)
- **[Terraform Workspaces](https://www.runatlantis.io/docs/terraform-workspaces.html)** - Using Terraform workspaces with Atlantis

### Deployment & Security

- **[Installation Guide](https://www.runatlantis.io/docs/installation-guide.html)** - Set up your Atlantis server
- **[Deployment](https://www.runatlantis.io/docs/deployment.html)** - Deploy Atlantis with Docker, Kubernetes, or other platforms
- **[Security](https://www.runatlantis.io/docs/security.html)** - Best practices for securing your Atlantis installation
- **[Webhook Secrets](https://www.runatlantis.io/docs/webhook-secrets.html)** - Secure your webhooks from unauthorized requests

### Troubleshooting & Support

- **[FAQ](https://www.runatlantis.io/docs/faq.html)** - Frequently asked questions
- **[Troubleshooting](https://www.runatlantis.io/docs/troubleshooting.html)** - Common issues and solutions
- **[Atlantis Community](https://www.runatlantis.io/community.html)** - Get help from the community

## Contributing

To test any changes you've made, run `make gotestsum` (or `make test` for standard golang testing).

When your PR is merged and a tag is created, a Github Actions job to build the new binary, test it, and deploy it's artifacts to Github Releases along with checksums.

You can then open a PR on our homebrew tap similar to https://github.com/transcend-io/homebrew-tap/pull/4, and as soon as that merges your code will be released. Homebrew is not updated for every release, as Github is the primary artifact store.

## Contributors

<img src="./CONTRIBUTORS.svg">

## Stargazers over time

[![Stargazers over time](https://starchart.cc/transcend-io/terragrunt-atlantis-config.svg)](https://starchart.cc/transcend-io/terragrunt-atlantis-config)

## License

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Ftranscend-io%2Fterragrunt-atlantis-config.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Ftranscend-io%2Fterragrunt-atlantis-config?ref=badge_large)
