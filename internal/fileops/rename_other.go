//go:build !linux

package fileops

// renameNoReplace renames from to to after checking that nothing is at to: this system
// has no rename that refuses to replace by itself.
func renameNoReplace(from, to string) error {
	return renameNoReplaceFallback(from, to)
}
