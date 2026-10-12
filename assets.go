package csw

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// MaxAssetTotal is the limit on all of a widget's assets together,
// decoded; it is MaxAssetsBytes.
const MaxAssetTotal = MaxAssetsBytes

var assetPathRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*$`)

// checkAssets validates a widget's assets: paths, types, sizes, counts and
// that the bytes look like the type they claim.
func checkAssets(m map[string]DefinitionAsset) error {
	if len(m) > MaxAssets {
		return invalidf("a widget has at most %d assets", MaxAssets)
	}
	types := AssetTypes()
	total := 0
	for _, p := range sortedKeys(m) {
		a := m[p]
		if err := checkAssetPath(p); err != nil {
			return err
		}
		if _, ok := types[a.Type]; !ok {
			return invalidf("asset %q has type %q, which is not an allowed image, sound or font type", p, a.Type)
		}
		if a.Data == nil && !validSHA(a.SHA256) {
			return invalidf("asset %q has no data", p)
		}
		n := a.length()
		if n < 1 {
			return invalidf("asset %q is empty", p)
		}
		if n > MaxAssetBytes {
			return invalidf("asset %q is larger than %d MB", p, MaxAssetBytes>>20)
		}
		if a.Data != nil && !sniffsAs(a.Type, a.Data) {
			return invalidf("asset %q is not a valid %s file", p, a.Type)
		}
		total += n
	}
	if total > MaxAssetTotal {
		return invalidf("assets total more than %d MB", MaxAssetTotal>>20)
	}
	return nil
}

// checkAssetPath checks one asset path.
func checkAssetPath(p string) error {
	if len(p) > MaxAssetPath || !assetPathRE.MatchString(p) || strings.Contains(p, "..") {
		return invalidf("asset path %q must be lowercase letters, digits, . _ - and / between names (at most %d characters)", p, MaxAssetPath)
	}
	return nil
}

func validSHA(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && s == strings.ToLower(s)
}

// sniffsAs reports whether data starts like a file of the media type.
func sniffsAs(typ string, data []byte) bool {
	has := func(prefix string) bool { return bytes.HasPrefix(data, []byte(prefix)) }
	switch typ {
	case "image/png":
		return has("\x89PNG\r\n\x1a\n")
	case "image/jpeg":
		return has("\xff\xd8\xff")
	case "image/gif":
		return has("GIF87a") || has("GIF89a")
	case "image/webp":
		return has("RIFF") && len(data) > 12 && string(data[8:12]) == "WEBP"
	case "audio/wav":
		return has("RIFF") && len(data) > 12 && string(data[8:12]) == "WAVE"
	case "audio/ogg":
		return has("OggS")
	case "audio/mpeg":
		return has("ID3") || (len(data) > 1 && data[0] == 0xff && data[1]&0xe0 == 0xe0)
	case "font/woff2":
		return has("wOF2")
	case "image/svg+xml":
		return bytes.Contains(data[:min(len(data), 4096)], []byte("<svg"))
	}
	return false
}
