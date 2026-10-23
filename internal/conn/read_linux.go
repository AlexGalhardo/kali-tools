//go:build linux

package conn

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func Read(ctx context.Context) (Table, error) {
	return readProc(ctx, "/proc")
}

func readProc(ctx context.Context, root string) (Table, error) {
	if err := ctx.Err(); err != nil {
		return emptyTable(), err
	}
	files := []struct{ name, protocol string }{
		{"tcp", "tcp"}, {"tcp6", "tcp6"}, {"udp", "udp"}, {"udp6", "udp6"},
	}
	var raws []rawRow
	saw := false
	for _, file := range files {
		body, err := readLimited(filepath.Join(root, "net", file.name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return emptyTable(), classify(err)
		}
		saw = true
		raws = append(raws, parseNet(string(body), file.protocol)...)
	}
	if !saw {
		return emptyTable(), domain.Unsupported("socket table is not available")
	}
	owners, limited, err := scanOwners(ctx, root)
	if err != nil {
		return emptyTable(), err
	}
	return assemble(raws, owners, limited), nil
}

func scanOwners(ctx context.Context, root string) (map[uint64]owner, bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, false, classify(err)
	}
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		pids = append(pids, pid)
	}
	slices.Sort(pids)
	limited := false
	if len(pids) > maxPIDs {
		pids = pids[:maxPIDs]
		limited = true
	}
	owners := map[uint64]owner{}
	for _, pid := range pids {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		name, fds, skip, err := processFDs(root, pid)
		if err != nil {
			return nil, false, err
		}
		if skip {
			continue
		}
		if len(fds) > maxFDs {
			fds = fds[:maxFDs]
			limited = true
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(root, strconv.Itoa(pid), "fd", fd.Name()))
			if err != nil {
				continue
			}
			inode, ok := socketInode(target)
			if !ok {
				continue
			}
			if prev, exists := owners[inode]; !exists || pid < prev.pid {
				owners[inode] = owner{pid: pid, name: name}
			}
		}
	}
	return owners, limited, nil
}

func processFDs(root string, pid int) (string, []os.DirEntry, bool, error) {
	base := filepath.Join(root, strconv.Itoa(pid))
	comm, err := readLimited(filepath.Join(base, "comm"))
	if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) {
		return "", nil, true, nil
	}
	if err != nil {
		return "", nil, false, classify(err)
	}
	fds, err := os.ReadDir(filepath.Join(base, "fd"))
	if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) {
		return "", nil, true, nil
	}
	if err != nil {
		return "", nil, false, classify(err)
	}
	name := strings.TrimSpace(string(comm))
	if len(name) > 64 {
		name = name[:64]
	}
	return name, fds, false, nil
}

func socketInode(target string) (uint64, bool) {
	value, ok := strings.CutPrefix(target, "socket:[")
	if !ok {
		return 0, false
	}
	value, ok = strings.CutSuffix(value, "]")
	if !ok {
		return 0, false
	}
	inode, err := strconv.ParseUint(value, 10, 64)
	if err != nil || inode == 0 {
		return 0, false
	}
	return inode, true
}

func readLimited(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxFileBytes {
		return nil, errTooLarge
	}
	return body, nil
}

var errTooLarge = errors.New("socket table exceeds 1 MiB")

func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrPermission) {
		return &domain.Error{Code: domain.CodePermissionRequired, Message: "reading the socket table needs permission"}
	}
	if errors.Is(err, os.ErrNotExist) {
		return domain.Unsupported("socket table is not available")
	}
	if errors.Is(err, errTooLarge) {
		return &domain.Error{Code: domain.CodeFailed, Message: "socket table exceeds 1 MiB"}
	}
	return err
}
