//go:build !unix && !windows

package storage

// processAlive reports false where liveness cannot be probed, so orphans are cancelled as before.
func processAlive(int) bool { return false }
