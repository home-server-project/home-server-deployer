# Alpha 0 destructive VM suite

This directory is intentionally destructive and is **not** for the production Home Server.

The harness exits unless this marker exists:

```text
/etc/home-server-deployer/test-vm
```

Use a disposable/dedicated uCore VM with local VM filesystems. Do not mount production media or production application data read/write into the VM.

The first real target is uCore/Fedora CoreOS 44 with Podman 5.8.4. The same suite will later be run on the Fedora 45/uCore Podman 6.1.x target.

Typical sequence:

```sh
sudo touch /etc/home-server-deployer/test-vm
./tests/vm/run-alpha0.sh preflight
./tests/vm/run-alpha0.sh install-deployer
./tests/vm/run-alpha0.sh smoke-install
./tests/vm/run-alpha0.sh rollback
./tests/vm/run-alpha0.sh resilience
# reboot the VM here, then:
./tests/vm/run-alpha0.sh verify-after-reboot
./tests/vm/run-alpha0.sh smoke-remove
```

`install-deployer` uses a VM-only copy of the catalog mounted read-only into Agent. This lets the `rollback` phase deliberately modify the test catalog to point at a missing image, prove rollback, then restore the test template. The normal deployment Quadlet and production catalog image remain unchanged.
