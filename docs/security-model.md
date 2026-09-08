# Alpha 0 security model

## Trust boundary

The rootful Podman socket is administrative/root-equivalent. Alpha 0 does not claim otherwise. The security objective is compartmentalization: browser input reaches an unprivileged Web process, while only the small Agent receives the privileged host interfaces.

## Web boundary

The Web container receives no Podman socket, systemd socket, Deployer state directory, or application-data root. It communicates with Agent only through the private shared Unix socket.

## Agent boundary

The Agent has no published TCP port. Its API is typed and limited to Deployer operations such as capability discovery, managed-application planning, lifecycle actions, logs, and generated documentation. Host objects are resolved from managed state rather than accepted as unrestricted browser-supplied identifiers.

The runtime Agent image is a scratch image containing only the static Go binary plus trusted catalog/configuration files.

## Approved roots

Application paths are represented as an approved-root ID plus a validated relative subpath. The administrator chooses the actual host roots outside the browser.

Directory creation uses Linux `openat2` resolution with `RESOLVE_BENEATH`, `RESOLVE_NO_SYMLINKS`, and `RESOLVE_NO_MAGICLINKS`. Absolute paths, parent traversal, and symlink escapes are rejected.

## SELinux

Catalogs describe storage intent rather than SELinux commands. Alpha 0 maps private/cache/database data to private relabel intent, shared data to shared relabel intent, and read-only media to read-only access without automatically relabeling the media tree.

The same application-level intent interface is intended to support an AppArmor backend later.

## Drift and plans

After a successful deployment, Deployer reads the installed Quadlet back through Podman and records its live SHA-256 baseline. Update and removal re-check that source and refuse to overwrite administrator drift.

Mutating catalog operations use a stored, expiring plan with a digest. Execution uses the plan ID rather than resubmitting a new host path or Quadlet definition.

## Known Alpha 0 limitations

- Agent authority remains root-equivalent because it controls the rootful Podman socket.
- Browser authentication is not part of Alpha 0; the supplied Web Quadlet binds only to `127.0.0.1:8080` for development/testing.
- Application-aware health providers and backup execution are later milestones.
- AppArmor support follows after the SELinux engine is proven.
