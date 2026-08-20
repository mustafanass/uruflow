# Upgrading

How to move between URUFLOW versions. Version 2.2 updates the UFP wire format and requires coordinated server
and agent replacement. Moving from 1.x remains a rebuild.

## From 2.0 or 2.1 to 2.2

1. wait for every release to finish
2. stop the server and all agents
3. replace `/usr/local/bin/uruflow` and every `/usr/local/bin/uruflow-agent`
4. replace mutable prebuilt image tags in project files with `repository@sha256:digest` references
5. start the server
6. start every agent
7. change git-host webhook URLs from `http://` to `https://`, unless a local reverse proxy terminates TLS

Database schema changes are applied automatically when the database is opened — missing columns are
added in place, so no migration step is needed and existing data is preserved.

The current UFP wire format uses protocol byte `0x03` and TLS 1.3. A 2.0 or 2.1 agent cannot connect to a 2.2 server, and a
2.2 agent cannot connect to an older server. Running containers are unaffected during the coordinated
upgrade.

Version 2.2 also binds roles to enrollment, deploys images by digest, snapshots project configuration
per release, rejects webhook replays, and serves webhooks over HTTPS by default.

## Within One Protocol Generation

Patch upgrades that retain the UFP wire format can use the normal server-first order: stop and replace the server,
then replace and restart each agent. A running process keeps the code it started with, and workloads
continue under Docker while the control plane is unavailable.

## From 1.x to 2.0

**2.0 is a clean break. There is no in-place migration.**

The two versions do not describe the same system. 1.x built from source on every target machine; 2.0
builds once and releases an image. Nothing about the wire protocol, the configuration layout or the
database schema carries across.

### What Changed

| | 1.x | 2.0 |
| :--- | :--- | :--- |
| Deployment unit | Source built per machine | One image, released everywhere |
| Protocol | UFP version `0x01`, 17 message types | UFP version `0x02`, three envelopes |
| Agent auth | Token sent on the wire | HMAC challenge-response; the key never travels |
| Transport | Optional TLS | TLS always |
| Registry | None | Bundled and managed |
| Agent roles | None | `builder` and `runner`, enforced |
| Configuration | Agents and repositories in `config.yaml` | Agents in the database, projects in the database or files |
| Rollback | Rebuild from a commit | Re-release a stored image |

A 1.x agent is rejected at the first frame because the version byte differs. There is no partial
compatibility to manage.

### Migration Steps

1. record your existing repositories, branches and target machines
2. stop the 1.x server and all 1.x agents
3. install the 2.0 binaries
4. `uruflow init` — write a fresh `config.yaml`; the old one is not read
5. start the server and let it generate its PKI and registry
6. re-enrol each agent with `uruflow agent add`, assigning roles deliberately: machines that only run
   workloads should get `runner` alone
7. install the 2.0 agent binary and configuration on each machine, copying the new CA certificate
8. re-create each project, either in the interface or as files
9. deploy each project once to populate the registry — there is no history to roll back to until you
   have a successful release

Keep the old data directory until you are satisfied. Nothing in 2.0 reads it.

### After Migrating

Two behaviours differ in ways worth knowing:

- **Runners no longer need source access.** Remove deploy keys and build tooling from machines that
  now hold only the `runner` role.
- **Rollback no longer rebuilds.** It re-releases a stored image, so it is fast and byte-identical —
  but only for commits that have been built and pushed since migrating.
