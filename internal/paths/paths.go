package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type Root struct {
	ID        string
	HostPath  string
	AgentPath string
}

func Resolve(root Root, relative string) (agentPath, hostPath string, err error) {
	if root.ID == "" || !filepath.IsAbs(root.AgentPath) || !filepath.IsAbs(root.HostPath) {
		return "", "", errors.New("invalid approved root")
	}
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != relative || relative == "." {
		return "", "", fmt.Errorf("path must be a clean relative path")
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." || strings.ContainsRune(part, '\x00') {
			return "", "", fmt.Errorf("unsafe path component %q", part)
		}
	}
	return filepath.Join(root.AgentPath, relative), filepath.Join(root.HostPath, relative), nil
}

// EnsureDir creates relative below root.AgentPath while refusing symlink, magic-link,
// absolute-path, and parent traversal escapes. Each directory transition is resolved
// by the kernel with openat2(RESOLVE_BENEATH|NO_SYMLINKS|NO_MAGICLINKS).
func EnsureDir(root Root, relative string, mode os.FileMode) (string, error) {
	agentPath, _, err := Resolve(root, relative)
	if err != nil {
		return "", err
	}
	base, err := unix.Open(root.AgentPath, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return "", fmt.Errorf("open approved root %q: %w", root.ID, err)
	}
	defer unix.Close(base)
	fd := base
	owned := false
	defer func() {
		if owned {
			_ = unix.Close(fd)
		}
	}()
	parts := strings.Split(relative, string(filepath.Separator))
	for _, part := range parts {
		if err := unix.Mkdirat(fd, part, uint32(mode.Perm())); err != nil && !errors.Is(err, unix.EEXIST) {
			return "", fmt.Errorf("mkdir %q: %w", part, err)
		}
		next, err := unix.Openat2(fd, part, &unix.OpenHow{
			Flags:   unix.O_PATH | unix.O_DIRECTORY | unix.O_CLOEXEC,
			Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
		})
		if err != nil {
			return "", fmt.Errorf("resolve %q: %w", part, err)
		}
		if owned {
			_ = unix.Close(fd)
		}
		fd, owned = next, true
	}
	return agentPath, nil
}
