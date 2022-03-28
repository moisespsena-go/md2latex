// Verbatim code from Blackfriday HTML renderer.

package pkg

import "bytes"

var validUris = [][]byte{[]byte("http://"), []byte("https://"), []byte("ftp://"), []byte("mailto://")}
var validPaths = [][]byte{[]byte("/"), []byte("./"), []byte("../")}

// Test if a character is letter.
func isletter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// Test if a character is a letter or a digit.
func isalnum(c byte) bool {
	return (c >= '0' && c <= '9') || isletter(c)
}

func isSafeLink(link []byte) bool {
	for _, path := range validPaths {
		if len(link) >= len(path) && bytes.Equal(link[:len(path)], path) {
			if len(link) == len(path) {
				return true
			} else if isalnum(link[len(path)]) {
				return true
			}
		}
	}

	for _, prefix := range validUris {
		// TODO: handle unicode here
		// case-insensitive prefix test
		if len(link) > len(prefix) && bytes.Equal(bytes.ToLower(link[:len(prefix)]), prefix) && isalnum(link[len(prefix)]) {
			return true
		}
	}

	return false
}

func isMailto(link []byte) bool {
	return bytes.HasPrefix(link, []byte("mailto:"))
}

func needSkipLink(flags Flag, dest []byte) bool {
	if flags&SkipLinks != 0 {
		return true
	}
	return flags&Safelink != 0 && !isSafeLink(dest) && !isMailto(dest)
}
