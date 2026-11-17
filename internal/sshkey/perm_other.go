//go:build !windows

package sshkey

// restrict is a no-op where 0600 already limits the file to its owner.
func restrict(string) {}
