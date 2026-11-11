# Projects

A project groups services, their sources and runtime settings into one release unit. Its authoritative definition is a YAML file, and its environment variables live beside it. This page covers files, creation, variables and safe editing. The complete schema lives in [Build and runtime model](native-build-model.md).

## Files are the source of truth

```text
/etc/uruflow/
  config.yaml
  defaults.yaml
  projects/
    shop/
      staging.yaml
      staging.env
      production.yaml
      production.env
```

The folder and YAML filename become `<project>-<environment>`, such as `shop-production`. Each YAML file is complete. The database holds its validated model and release history; saved releases keep the specification used for that attempt.

Project, environment and service names use lowercase letters, digits, `.`, `_` and `-`, start with a letter or digit, and fit the resource-name validation. The combined project name must also fit its 63-character bound.

## Create a project

Choose **Projects → Create project**. Guided setup collects the name, environment, service source and placement. **Environment YAML** accepts a complete definition.

For a source build:

```yaml
workflow: build_deploy
builder: build-01
runners: [runner-01]
timeout: 2h
services:
  api:
    git: git@github.com:your-org/shop.git
    branch: main
    dockerfile: Dockerfile
    context: .
    ports: ["8080:8080"]
    healthcheck:
      type: http
      path: /health
      port: 8080
```

Replace the repository, agent names and application port with your own. Agent names must resolve to enrolled machines with the required roles. For a prebuilt service, use an immutable image reference in `image` instead of build fields. Workflow rules and field definitions are in the [model reference](native-build-model.md#project-fields).

After creation, inspect the **Overview**, configure variables, then choose **Deploy**. Saving configuration does not start a release.

## Variables

Project variables merge in this order; later entries replace earlier ones:

```text
defaults.yaml env → environment YAML env → environment .env → service env
```

`defaults.yaml` is optional and sits beside `config.yaml`:

```yaml
env:
  TZ: Asia/Baghdad
  LOG_LEVEL: info
```

A project's `production.env` can override those values:

```dotenv
LOG_LEVEL=warn
APP_HOST=shop.example.com
```

The dotenv parser accepts comments, `export`, quoted values and escaped newlines. Invalid assignments identify their line. Service variables apply only to that service.

Runtime fields support `${NAME}`, `${NAME:-default}`, `${NAME:?message}` and `$$` for a literal dollar sign. Required interpolation values must be supplied before a document can load. `${secret:name}` remains a reference until release dispatch.

## Protected values

Open the variables editor in **Configuration**. Plain values use `NAME=value`. Prefix a value with `secret ` to store it encrypted:

```text
LOG_LEVEL=info
secret DATABASE_URL=postgres://user:password@database/shop
```

For `shop-production`, saving writes this reference to `production.env`:

```dotenv
LOG_LEVEL=info
DATABASE_URL=${secret:shop-production.DATABASE_URL}
```

The editor reopens it as `secret DATABASE_URL=${secret:shop-production.DATABASE_URL}`. Replace the reference with a new value to rotate it. Ordinary variables remain readable configuration. [Security](security.md#secrets) explains storage and resolution.

## Edit safely

**Configuration** contains separate editors for the authoritative YAML and variable overlay. **Review changes** shows the diff, and **Save changes** validates the resolved document and its revision before writing.

A validation or permission error preserves your text. If another operator has saved first, copy your edits and choose **Load latest** to obtain the current document and revision. Apply your changes to that version and save again.

Unsaved document changes block navigation. Graph positions save automatically in the browser and do not affect configuration state. Saving one document preserves the other editor’s unsaved state.

## Load external changes

Files are loaded on server startup and by **Projects → Reload definitions**. Use reload after a configuration-management system changes YAML, variable overlays or shared defaults.

Loading reports invalid files and missing definitions. A removed definition does not implicitly remove workloads: restore its file and reload to reconcile the desired configuration. To remove a project instead, delete it from the workspace.

## Delete a project

Administrators delete one environment from **Settings → Delete project**. Other environments of the same project are not affected.

Deletion waits until no release is running anywhere and every runner of the project is online, so nothing is left running on a machine the server cannot reach. It then removes the project's containers and its own networks from every runner, its environment and variable files, its release history and logs, its secrets, deploy-on-push sources and alerts, and the project from every account's assignments. The audit log records `project.delete`.

Data volumes and registry images are kept unless they are selected in the dialog. Selected volumes are removed from every runner and need agents on 3.0.6 or later. Selected images are removed from the registry, and their unreferenced layers are collected. Volumes, networks and images that another project also uses are always kept, as are external resources and manually tagged images. A failed deletion can be repeated; files are removed before database records, so a partial deletion never restores the project.

## Implementation

`internal/projects` owns parsing, interpolation, schema validation and file writes. `internal/ops/documents.go` owns revision checks and save coordination. `internal/web` checks access before operations, including resolved agent placement and secret references.

Related: [Deployments](deployments.md), [Browser workspace](interface.md), [Configuration](configuration.md).
