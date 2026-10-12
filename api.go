package csw

import "regexp"

var (
	api3RE = regexp.MustCompile(`\bsl\.asset\b`)
	api2RE = regexp.MustCompile(`\bsl\.(alert|leave|sfx)\b`)
)

// DetectAPI returns the lowest API level the widget's script needs: 3 if it
// calls sl.asset or ships assets, 2 if it uses sl.alert, sl.leave or sl.sfx, else 1. It is a
// text scan, so a mention in a comment counts.
func DetectAPI(p *Package) int {
	switch {
	case len(p.Assets) > 0 || api3RE.MatchString(p.Code.JS):
		return APIAssets
	case api2RE.MatchString(p.Code.JS):
		return APIAlerts
	}
	return APIBase
}
