# Alpha 0 scope and acceptance criteria

Alpha 0 is the engine/privilege-boundary proof. UI polish and real application catalog breadth are explicitly out of scope.

## Acceptance criteria

| ID | Requirement |
|---|---|
| A0-01 | Build two OCI images, `deployer-web` and `deployer-agent`, with no RPM/DEB packaging. |
| A0-02 | Web is unprivileged and receives no Podman socket, systemd socket, Deployer state directory, or application-data root. |
| A0-03 | Agent exposes no TCP listener, generic shell/API proxy, broad host mount, or raw host-management endpoint; Web/Agent communication uses a private Unix socket. |
| A0-04 | Capability discovery reports Podman engine/Libpod API, rootful/rootless, Quadlet API availability, cgroup manager/version, network backend, architecture, systemd, SELinux, and approved roots. Capability introduction points are modeled separately from Home Server Deployer's product support policy. |
| A0-05 | Existing system Quadlets are discovered through native Podman Quadlet APIs and external/unmanaged definitions remain read-only. |
| A0-06 | Runtime containers correlate to generated services through `PODMAN_SYSTEMD_UNIT`, not container-name guessing. |
| A0-07 | Application directories can be created only below pre-mounted approved roots; traversal/symlink/magic-link escapes are rejected. |
| A0-08 | SELinux intent is translated without exposing arbitrary SELinux administration. |
| A0-09 | Install/update/remove create a deterministic expiring plan containing affected resources/paths, hashes, drift and a plan digest before execution. |
| A0-10 | Smoke application installs through the native Quadlet REST API with individual Quadlets and a deferred single systemd reload, then starts through systemd. |
| A0-11 | Start/stop/restart is available only for units resolved from a managed instance; no raw unit-name lifecycle endpoint exists. |
| A0-12 | Status combines Quadlet/systemd/runtime state and exposes native Podman health state where available; logs come from mapped runtime containers. |
| A0-13 | Update snapshots the current managed source, stops managed units, removes old Quadlets with reload deferred, installs the new definitions normally with reload deferred, performs one daemon reload, then starts/restarts and verifies the resolved units. Native Quadlet `replace=true` is not used in Alpha 0. Administrator drift causes refusal instead of overwrite. |
| A0-14 | A deliberately failing update restores the previous live Quadlet source without native replace, reloads systemd once, and returns the previous service to a verified working state; rollback failure is surfaced as a hard error. |
| A0-15 | Remove stops/removes only managed Quadlets and preserves application data by default. |
| A0-16 | The deployed smoke application continues through native systemd/Podman when Deployer is stopped and survives VM reboot. |
| A0-17 | Successful lifecycle mutations preserve persistent Markdown/JSON recovery documentation; removal retains historical documentation. Documentation root is configurable, defaulting to `/var/lib/home-server-deployer/docs`. |
| A0-18 | A versioned application-level backup contract exists without executing arbitrary catalog shell hooks; execution remains intentionally unimplemented. |
| A0-19 | Browser reaches Web only; Web reaches Agent by UDS; browser mutations require CSRF tokens; malformed/raw host-management inputs are rejected. |
| **A0-20** | **Podman 5.8.2-6.1.x compatibility:** Home Server Deployer's supported floor is 5.8.2. Quadlet REST capability detection recognizes the API family from its Podman 5.8.0 introduction point rather than conflating API introduction with product support. Contract fixtures/tests cover Podman 5.8.2, 5.8.4, and 6.1.x. The engine keeps a stable backend abstraction, uses individual Quadlets + deferred reload + safe remove/install as the Alpha 0 common denominator, and does not depend on Podman's private application storage representation. |
| A0-21 | Destructive integration tooling refuses to run without the dedicated test-VM marker and uses only VM-local data. The harness itself avoids native Quadlet replace, defers reload during individual Deployer-Quadlet operations, and is safely repeatable after a partial/previous harness install. |
| A0-22 | CI runs formatting/vet/unit/contract/security tests and builds both OCI images. Alpha 0 is considered Podman-compatible only after the same core destructive VM suite, including the shorter-Quadlet regression, passes sequentially on Fedora 44/Podman 5.8.4, Fedora 45/Podman 6.1.x, and AlmaLinux 10.2/Podman 5.8.2. |

## Version test policy

Current day-one compatibility targets are:

| Platform | Podman | Required Alpha 0 validation |
|---|---:|---|
| AlmaLinux 10.2 | 5.8.2 | full core VM suite |
| Fedora 44 / current uCore | 5.8.4 | full core VM suite |
| Fedora 45 | 6.1.x | full core VM suite |

The suites may be run sequentially and do not need to be GitHub-hosted. All three must pass before Alpha 0 is described as Podman-compatible across the declared range.

CI contract tests retain separate Podman 5.8.2, 5.8.4, and 6.1.0 version/API fixtures. Capability tests also prove that the Quadlet REST capability family begins at 5.8.0 while the Deployer-supported product floor remains 5.8.2.

## Native replace safety

Podman 5.7.0 through 5.8.5 are affected by GHSA-fx76-2j3w-2mx6 / CVE-2026-19730: `podman quadlet install --replace` may fail to truncate a longer previous file when installing a shorter definition. The fix is in Podman 5.8.6; Podman 6.x is not affected.

Alpha 0 therefore does not use native replace on any supported Podman version. A regression test installs a longer Quadlet containing an extra trailing directive, updates it to a shorter definition, and verifies that the removed directive is absent after the safe remove + normal-install transaction.
