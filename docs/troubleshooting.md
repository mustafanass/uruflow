# Troubleshooting

Start with the affected resource and its latest observation. Release outcomes, current container state and connection status answer different questions. Use the browser message and the matching process log together.

## Find the evidence

| Evidence                            | Location                                                    |
| ----------------------------------- | ----------------------------------------------------------- |
| Server process                      | `<data_dir>/uruflow.log` or `journalctl -u uruflow`         |
| Agent process                       | `log_file` in `agent.yaml` or `journalctl -u uruflow-agent` |
| Build, job and rollout output       | Release → Logs                                              |
| Runtime output                      | Container → Logs                                            |
| Host and container observation time | Agents, Containers and System                               |
| User actions and access changes     | System → Audit log, for administrators                      |

## Browser access and sign-in

**The workspace does not open.** Check that `uruflow serve` is running and use `web.host` and `web.port`. The default is `127.0.0.1:9002`. The webhook listener is a separate service address. A non-loopback operator bind requires HTTPS.

**The page loads but stays reconnecting.** Check server availability and reverse-proxy streaming settings. SSE must pass without buffering. The page preserves its last observation while reconnecting; verify that fresh samples resume after the service returns.

**Sign-in appears again.** Check session expiry and account changes. Valid sessions survive server restarts, while password, permission and explicit session revocation require another sign-in. [Security](security.md#passwords-and-sessions) owns the policy.

**The only administrator's password is lost.** Run the local [administrator recovery command](security.md#administrator-recovery) against the existing server configuration.

**A project is missing for one user.** An administrator should check its exact expanded name under **System → Access**. A role alone does not grant non-admin accounts project access.

## Server startup

**Configuration validation fails.** Read the field named in the error. Confirm advertise and registry names, generated credentials, distinct operator port and certificate pairs using [Configuration](configuration.md).

**Docker cannot be reached.** Check the daemon, `registry.socket` and the service account's socket access.

**The registry cannot start or become healthy.** Check its published port, storage permissions and `docker logs uruflow-registry`. Confirm the configured registry name reaches that host.

**CA or secret material is missing.** Restore the matching PKI and database from the same backup. The server checks for missing keys before starting against existing enrollment or encrypted values.

**Frontend assets are missing.** Build with `make build` and install its server binary. The production build embeds the browser assets.

## Agent connection

**Enrollment or key rejection.** Compare `agent_id`, key and roles with that enrollment. If the original command was lost, enroll a replacement from **Agents** and use its new configuration.

**Unknown certificate authority.** Download the CA from **Agents**, install it at `server.ca_cert` on the agent, and restart the agent.

**Wrong certificate name.** Check that the agent reaches the intended server and that its certificate belongs to this installation. Restore or repair the matching server PKI rather than replacing the CA independently.

**Connection refused or timed out.** Check the agent's server address, UFP port, server bind and firewall path. The agent initiates the connection outward.

**Registry trust or login fails.** Check the agent log, certificate-directory permissions and builder Docker CLI access. Registry readiness must complete before build or release dispatch.

## Project files and saving

**An image tag is rejected.** Supply the full `repository@sha256:digest` reference. The model requires 64 lowercase hexadecimal digest digits.

**An agent name or role is rejected.** Use enrolled names shown on **Agents**, with builder and runner roles matching the workflow.

**An unknown YAML field is reported.** Check the field spelling and placement against [Build and runtime model](native-build-model.md).

**A save conflicts.** Copy your unsaved edits, choose **Load latest**, confirm the replacement, then reapply changes to the fresh document. During a conflict, **Discard edits** also offers this path.

**External edits are absent.** Choose **Projects → Reload definitions** after changing files. If a definition disappeared, restore it and reload; running workloads are not implicitly deleted.

**A required variable or secret is missing.** Supply the interpolation value or replace the affected secret reference in the variables editor under **Configuration**, then save. [Projects](projects.md) explains the syntax.

## Release execution

**Another release owns the project.** Inspect that release's progress and target outcomes before starting new work. The project permits one active release.

**Build or pull fails.** Read the stage log. Check source access on the builder, base-image access, image identity, registry availability and the relevant agent's network.

**The old version is running after failure.** Inspect startup output and readiness errors in the failed release. The runner attempts to restore the previous container when replacement fails.

**Some runners succeeded and another failed.** Read each target's message and current inventory. Fix the failed target and deploy again, or deliberately restore an earlier successful release.

**A release says it was interrupted by restart.** Startup reconciled unfinished work as failed. Inspect current containers and fresh agent observations before the next attempt.

**Restore is unavailable.** Confirm the project has an eligible successful release with artifacts. The release already identified as live is not offered as a restore target.

## Containers and logs

**A succeeded release now has an exited container.** The container may have stopped after rollout readiness. Open its health details and logs; compare timestamps with the release result.

**A failed replacement disappeared.** Its startup output is copied into the release log before cleanup. Job output also belongs to the release log.

**A viewed container leaves inventory.** Keep its open detail view to inspect the last observation and loaded output. Download loaded logs before leaving that view.

**No log lines appear.** Check agent connectivity and the viewer's state or error. Docker output comes from stdout and stderr; applications writing only internal log files need their own log collection. An idle application may simply have no new output.

**Metrics are unavailable or old.** Check collection timestamps, reported availability and the agent connection. An unavailable counter does not imply zero use. Wait for a fresh observation after reconnect.

## Describe a problem

Include `uruflow version`, `uruflow-agent version`, the platform, resource identity, expected result, actual result, relevant timestamps and selected log lines. Remove credentials and sensitive application output before sharing.

Related: [Operations](operations.md), [Security](security.md), [Deployments](deployments.md).
