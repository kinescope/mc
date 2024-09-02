//go:build appengine
// +build appengine

package mc

func str2byte(s string) []byte {
	return []byte(s)
}
