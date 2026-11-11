# Security

URUFLOW authenticates people at the operator API and agents at the UFP link. This page explains access decisions, credentials, protected state and administrator recovery.

## Trust boundaries

| Boundary                         | Verification                                                                    |
| -------------------------------- | ------------------------------------------------------------------------------- |
| Browser to operator API          | Local account, session cookie, role, project grant and CSRF token for mutations |
| Agent to server                  | TLS server verification, enrolled identity, HMAC challenge and exact role match |
| Builder or runner to registry    | TLS trust root and registry credentials                                         |
| Git provider to webhook listener | Provider signature or secret, push-event headers and delivery ID                |
| Server storage                   | Private database and key files; encrypted secret values                         |

Restrict host access to the server and agent machines. The server controls fleet deployment, while agent Docker access controls local workloads. Assign source access to builders and project operations to people trusted to run code on the selected hosts.

## Browser accounts and permissions

The first administrator is created with the private `<data_dir>/web-setup-token` file. Bootstrap works only before any account exists, and its token is removed after setup. Administrators manage accounts in **System → Access**.

| Role          | Actions                                                                                                                                   |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Viewer        | Read granted projects, runtime observations, releases and logs                                                                            |
| Operator      | Viewer actions plus editing, deployment, restoration, stopping, container restarts, container terminals and alert resolution within scope |
| Administrator | Fleet access, agent enrollment and removal, account management, system settings, audit access and registry deletion                       |

Non-admin accounts receive explicit project names; `*` grants fleet access. Creating projects and reloading definitions require fleet access as well as an operator role. Registry inspection requires fleet access. Changing a browser URL or resource ID does not bypass server checks.

The server filters snapshots, graphs, agents, workload logs and release history before transport. Document saves validate the resolved model before writing, including agent placement and secret references. An account change revokes affected sessions, and the last administrator is protected from removal or demotion.

## Passwords and sessions

Passwords are stored using bcrypt. New passwords require at least 15 characters and fit bcrypt's 72-byte bound. Sign-in uses generic errors and bounded attempts.

Sessions use random identifiers in HttpOnly, SameSite cookies. HTTPS uses Secure cookies. State-changing requests also require the session's CSRF token and pass origin protection. The operator listener defaults to loopback; [Configuration](configuration.md#operator-listener) explains HTTPS exposure.

Only hashes of session identifiers are stored in the private database. Valid sessions survive a server restart. Sessions expire after 30 idle minutes or eight hours total. Password or permission changes invalidate their credential fingerprint. Live streams recheck authentication and close revoked access.

Every user can change their password using the current password, or choose **Sign out** in the top-bar account menu to end their sessions on all devices. Administrators can reset another account's password through **System → Access**.

## Administrator recovery

On the server host, use the existing installation configuration:

```sh
sudo uruflow --config /etc/uruflow/config.yaml admin reset-password admin
```

The command opens the existing database, generates a replacement password, prints it locally, revokes the administrator's sessions and records the recovery in the audit log. It does not start another server. Recovery is restricted to administrator accounts.

Use `--password-stdin` to provide a replacement through private standard input. Passwords are not accepted as command-line arguments. Protect shell output and any password-manager input used for recovery.

## Agent identity and UFP

Enrollment creates an identity, shared key and roles. The one-time browser setup command carries those values into the local private agent configuration. Save it before dismissing setup. The CA certificate remains downloadable from **Agents**.

Agents verify the server certificate against the URUFLOW CA and logical name `uruflow-server`. The server sends a fresh random nonce, and the agent answers with `HMAC-SHA256(key, context || nonce)`. Verification uses constant-time comparison. The shared key is not transmitted during this handshake.

Enrolled roles must match the handshake declaration. The server checks roles before dispatch, and the agent checks them before execution. [UFP](protocol.md) owns exact frame, message and timing definitions.

## Registry and image identity

The managed registry uses a URUFLOW-signed certificate and bcrypt-backed `htpasswd` authentication. Registry configuration reaches agents over authenticated UFP. Agents install the Docker trust root; builders log in using standard input for the password.

Registry credentials are attached only to images belonging to that registry. Prebuilt images and built artifacts are dispatched by digest. Builder results must match the expected repositories and services before the pipeline accepts them.

## Secrets

The variables editor in project **Configuration** accepts `secret NAME=value`. Saving stores encrypted material and writes a `${secret:project-environment.NAME}` reference to the variable file. Saved values reopen as references. [Projects](projects.md#protected-values) owns entry and rotation syntax.

Secret values use AES-256-GCM with a fresh nonce. The key is `<data_dir>/pki/secrets.key`, mode `0600`. References are resolved at release dispatch; missing values fail before a build starts.

Resolved runtime values travel through TLS to Docker on the runner. They are available to the application and host administrators, including through Docker inspection. Keep application output free of credentials. Plain YAML and dotenv values remain ordinary readable configuration.

Back up the encrypted database and its matching encryption key together. The certificate authority also belongs to that backup set; see [Operations](operations.md#backup-and-restore).

## Webhooks and audit

GitHub requests use HMAC-SHA256 over the raw body. GitLab requests use a constant-time token comparison. The webhook listener verifies event headers and claims delivery IDs before handling a push. The webhook secret, any uploaded certificate key and the registry password are encrypted with the secret vault and kept outside project secrets, so project configuration cannot reference them. Rotating the registry password sends the new login to connected agents over UFP. A regenerated secret is shown once; the previous secret is accepted for 24 hours. [Configuration](configuration.md#webhooks) owns setup and routing behavior.

Operator mutations require an audit intent record before execution and then record their outcome. Container terminal sessions record the same intent when they open and their outcome when they end. Authentication attempts, access changes and local administrator recovery also leave audit records. Administrators inspect them in **System → Audit log**.

## Implementation

`internal/web` owns browser authentication and authorization. `internal/storage/sqlite` stores users, sessions and audit entries. `internal/ufp` and `internal/link` own agent authentication. `internal/pki`, `internal/secrets` and `internal/registry` own trust and protected material.

Related: [Architecture](architecture.md), [Operations](operations.md), [Testing](testing.md).
