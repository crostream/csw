package csw

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// invalidf returns an error wrapping ErrInvalid.
func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Validate reports the first rule p breaks; the error wraps ErrInvalid. It
// applies the widget rules (name, size, schema, defaults, code, data,
// sample, alert and assets, see ValidateDefinition) plus the format's: the
// ID, version and API level, the author and license, and the API level the
// assets need. It does not change p.
func (p *Package) Validate() error {
	if err := p.validate(); err != nil {
		return fmt.Errorf("csw: %w", err)
	}
	return nil
}

func (p *Package) validate() error {
	if p.CSW != Format {
		return invalidf("format version %d is not supported (want %d)", p.CSW, Format)
	}
	if err := p.checkIdentity(); err != nil {
		return err
	}
	if len(p.Assets) > 0 && p.API < APIAssets {
		return invalidf("a widget with assets needs api %d (it declares %d)", APIAssets, p.API)
	}
	if err := p.checkAssetExts(); err != nil {
		return err
	}
	d := p.definition()
	return ValidateDefinition(&d)
}

// checkAssetExts checks that each asset's file extension is one its media
// type may carry. Assets of unknown types are left to ValidateDefinition.
func (p *Package) checkAssetExts() error {
	types := AssetTypes()
	for _, name := range sortedKeys(p.Assets) {
		a := p.Assets[name]
		if exts, ok := types[a.Type]; ok && !slices.Contains(exts, strings.ToLower(path.Ext(name))) {
			return invalidf("asset %q does not end in %s, as its type %s requires", name, strings.Join(exts, " or "), a.Type)
		}
	}
	return nil
}

// definition returns a deep copy of the part of p that ValidateDefinition
// covers.
func (p *Package) definition() Definition {
	d := Definition{
		Name: p.Name, Description: p.Description, Icon: p.Icon, Tags: slices.Clone(p.Tags),
		Size: p.Size, KeepAspect: p.KeepAspect, Fields: cloneSchema(p.Fields), Defaults: slices.Clone(p.Defaults),
		Data: slices.Clone(p.Data), Sample: cloneRawMap(p.Sample), Alert: cloneAlert(p.Alert),
		HTML: p.Code.HTML, CSS: p.Code.CSS, JS: p.Code.JS,
	}
	if p.Assets != nil {
		d.Assets = make(map[string]DefinitionAsset, len(p.Assets))
		for name, a := range p.Assets {
			d.Assets[name] = DefinitionAsset{Type: a.Type, Data: slices.Clone(a.Data)}
		}
	}
	return d
}

func cloneRawMap(m map[string]json.RawMessage) map[string]json.RawMessage {
	if m == nil {
		return nil
	}
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		out[k] = slices.Clone(v)
	}
	return out
}

// checkIdentity checks the ID, version, API level, author, license and
// category.
func (p *Package) checkIdentity() error {
	if len(p.ID) > MaxIDLength || !idRE.MatchString(p.ID) {
		return invalidf("id %q must be lowercase dotted words (like crostream.alert-neon-burst), at most %d characters", p.ID, MaxIDLength)
	}
	if !semverRE.MatchString(p.Version) || len(p.Version) > 32 {
		return invalidf("version %q must be MAJOR.MINOR.PATCH", p.Version)
	}
	if p.API < APIBase || p.API > MaxAPI {
		return invalidf("api %d must be %d to %d", p.API, APIBase, MaxAPI)
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Author.Name)); n < 1 || n > maxAuthorName {
		return invalidf("author name must be 1 to %d characters", maxAuthorName)
	}
	if p.Author.URL != "" {
		u, err := url.Parse(p.Author.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(p.Author.URL) > maxAuthorURL {
			return invalidf("author url %q must be an http(s) URL of at most %d characters", p.Author.URL, maxAuthorURL)
		}
	}
	if n := utf8.RuneCountInString(p.License); n < 1 || n > maxLicense {
		return invalidf("license must be 1 to %d characters (an SPDX id or \"proprietary\")", maxLicense)
	}
	if len(p.Category) > maxCategory || !categoryRE.MatchString(p.Category) {
		return invalidf("category %q must be lowercase words joined by hyphens, at most %d characters", p.Category, maxCategory)
	}
	return nil
}

// Warnings lists problems that don't make p invalid but that its author
// should fix: a declared API level that is too low for the code (the
// widget would break) or higher than it needs (it would be refused by
// older apps for nothing), and assets the code never mentions.
func (p *Package) Warnings() []string {
	var out []string
	switch need := DetectAPI(p); {
	case p.API < need:
		out = append(out, fmt.Sprintf("declares api %d but its script needs api %d", p.API, need))
	case p.API > need:
		out = append(out, fmt.Sprintf("declares api %d but its script needs only api %d", p.API, need))
	}
	code := p.Code.HTML + "\n" + p.Code.CSS + "\n" + p.Code.JS
	for _, name := range sortedKeys(p.Assets) {
		if !strings.Contains(code, name) {
			out = append(out, fmt.Sprintf("asset %q is never mentioned in the code", name))
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
