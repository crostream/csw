package csw

import (
	"slices"
	"strings"
)

// Overlay themes: a small palette and two fonts per overlay. Any color
// prop (and any color inside effects and frames) may name a theme color
// as "theme:<key>", and any font prop a theme font as "theme:heading" or
// "theme:body"; the page resolves them, so changing the theme re-skins the
// overlay.

// ThemePrefix starts a theme reference.
const ThemePrefix = "theme:"

// ThemeColorKeys are the theme's colors, in the editor's order.
func ThemeColorKeys() []string {
	return []string{"primary", "secondary", "accent", "text", "muted", "panel", "border", "highlight"}
}

// ThemeFontKeys are the theme's fonts.
func ThemeFontKeys() []string { return []string{"heading", "body"} }

// isThemeColor reports whether s is a reference to a theme color.
func isThemeColor(s string) bool {
	k, ok := strings.CutPrefix(s, ThemePrefix)
	return ok && slices.Contains(ThemeColorKeys(), k)
}

// isThemeFont reports whether s is a reference to a theme font.
func isThemeFont(s string) bool {
	k, ok := strings.CutPrefix(s, ThemePrefix)
	return ok && slices.Contains(ThemeFontKeys(), k)
}

// validFontName reports whether s is a usable CSS font family list: a
// literal one, or a theme reference.
func validFontName(s string) bool {
	if strings.HasPrefix(s, ThemePrefix) {
		return isThemeFont(s)
	}
	return len(s) <= 200 && !strings.ContainsAny(s, ";{}<>\\")
}
