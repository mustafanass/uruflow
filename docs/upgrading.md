# Upgrading

How to move between URUFLOW versions. Upgrades within version 2 are routine; moving from 1.x is a
rebuild, because the two versions do not describe the same system.

## Within Version 2

1. **[server]** stop the service
2. replace `/usr/local/bin/uruflow`
3. start the service
4. **[target]** replace `/usr/local/bin/uruflow-agent` on each machine and restart the agent

Database schema changes are applied automatically when the database is opened — missing columns are
added in place, so no migration step is needed and existing data is preserved.

A running process keeps the code it started with. Replacing a binary changes nothing until you restart
the process.

### Order

Upgrade the server first. Agents reconnect on their own and tolerate a brief outage; their containers
keep running throughout.

Do not upgrade during a release. Wait for in-flight releases to settle, or expect them to be closed as
failed by the reconciliation that runs at startup.

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
