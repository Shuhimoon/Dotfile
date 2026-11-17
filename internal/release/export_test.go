package release

// SetZigKeyForTest replaces the trusted zig minisign key and returns a restore func.
func SetZigKeyForTest(k string) func() {
	old := zigPublicKey
	zigPublicKey = k
	return func() { zigPublicKey = old }
}
