package csw

import (
	"fmt"
	"regexp"
	"strings"
)

// Prefixes of the app's record IDs that a widget's media and folder values
// carry: a lowercase prefix, then a ULID in Crockford base32.
const (
	prefixMedia  = "media"
	prefixFolder = "folder"
)

var ulidRE = regexp.MustCompile(`^[a-z]+-[0-9a-hjkmnp-tv-z]{26}$`)

// checkID reports whether s is a well-formed ID with the given prefix.
func checkID(s, prefix string) error {
	if !ulidRE.MatchString(s) || !strings.HasPrefix(s, prefix+"-") {
		return fmt.Errorf("%w: %q (want %s-<ulid>)", ErrInvalid, s, prefix)
	}
	return nil
}
