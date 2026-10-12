package csw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Limits of a widget's code, schema and data.
const (
	MaxWidgetCode   = 256 << 10 // HTML, CSS and JS each
	MaxWidgetFields = 64
	MaxWidgetData   = 32 // data keys or prefixes it reads
	MaxWidgetSample = 256 << 10
	// MaxSize is the largest width or height of a widget's default size.
	MaxSize = 16_384
)

// Definition is the part of a widget that validation covers: what a .csw
// file holds besides its identity (ID, version, API level, author, license
// and category).
type Definition struct {
	Name, Description string
	Icon              string
	Tags              []string
	Size              Size
	KeepAspect        bool
	Fields            []Prop
	Defaults          json.RawMessage
	Data              []string
	Sample            map[string]json.RawMessage
	Alert             *WidgetAlert
	HTML, CSS, JS     string
	Assets            map[string]DefinitionAsset
}

// DefinitionAsset is one file a widget ships with. Data may be empty when
// SHA256 and Size describe the content instead, as for a stored widget.
type DefinitionAsset struct {
	Type   string
	Data   []byte
	SHA256 string // the hex digest of the content
	Size   int    // the length of the content in bytes
}

// length is the asset's size in bytes, whether or not its bytes are loaded.
func (a DefinitionAsset) length() int {
	if a.Data != nil {
		return len(a.Data)
	}
	return a.Size
}

// ValidateDefinition normalizes the parts of a widget that don't depend on
// where it is kept (name, text, size, schema, code, data, sample, alert and
// assets) in place and reports the first rule they break; the error wraps
// ErrInvalid.
func ValidateDefinition(d *Definition) error {
	if err := checkWidgetText(d); err != nil {
		return err
	}
	if d.Size.W < 1 || d.Size.W > MaxSize || d.Size.H < 1 || d.Size.H > MaxSize {
		return invalidf("size must be 1 to %d in each direction", MaxSize)
	}
	canonDefinition(d)
	if err := checkSchema(d.Fields, "field", MaxWidgetFields, 0); err != nil {
		return err
	}
	if err := checkWidgetDefaults(d); err != nil {
		return err
	}
	for name, code := range map[string]string{"HTML": d.HTML, "CSS": d.CSS, "JS": d.JS} {
		if len(code) > MaxWidgetCode {
			return invalidf("%s is longer than %d bytes", name, MaxWidgetCode)
		}
	}
	if err := checkWidgetData(d.Data); err != nil {
		return err
	}
	if err := checkWidgetSample(d.Sample); err != nil {
		return err
	}
	if len(d.Assets) == 0 {
		d.Assets = nil
	}
	if err := checkAssets(d.Assets); err != nil {
		return err
	}
	return checkWidgetAlert(d.Alert, d.Fields)
}

// canonDefinition replaces nil lists and maps with empty ones, so a
// definition always encodes as the same JSON shape.
func canonDefinition(d *Definition) {
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.Fields == nil {
		d.Fields = []Prop{}
	}
	if d.Data == nil {
		d.Data = []string{}
	}
	if d.Sample == nil {
		d.Sample = map[string]json.RawMessage{}
	}
	if len(d.Defaults) == 0 || string(d.Defaults) == "null" {
		d.Defaults = json.RawMessage("{}")
	}
}

// cloneSchema deep-copies a list of prop schemas.
func cloneSchema(ps []Prop) []Prop {
	if ps == nil {
		return nil
	}
	out := make([]Prop, len(ps))
	for i, p := range ps {
		p.Options = slices.Clone(p.Options)
		p.Accept = slices.Clone(p.Accept)
		p.Item = cloneSchema(p.Item)
		out[i] = p
	}
	return out
}

// Limits of a widget's descriptive text and schema.
const (
	maxWidgetDescription = 500
	maxWidgetIcon        = 40
	maxWidgetTags        = 10
	maxWidgetTag         = 30
	maxFieldKey          = 40
	maxFieldLabel        = 80
	maxFieldHelp         = 500
	maxFieldSection      = 60
	maxFieldUnit         = 20
	maxFieldOptions      = 64
	maxItemFields        = 32
	maxItemsMax          = 1000
	maxPropMaxLen        = 100_000
	maxBound             = 1e9
	maxWidgetKey         = 100
)

var (
	fieldKeyRE = regexp.MustCompile(`^[a-z][a-zA-Z0-9_]*$`)
	dataKeyRE  = regexp.MustCompile(`^[a-z][a-z0-9_.]*(\.\*)?$`)
)

func checkWidgetText(w *Definition) error {
	w.Name = strings.TrimSpace(w.Name)
	if n := utf8.RuneCountInString(w.Name); n < 1 || n > maxWidgetName {
		return invalidf("name must be 1 to %d characters", maxWidgetName)
	}
	if utf8.RuneCountInString(w.Description) > maxWidgetDescription {
		return invalidf("description must be at most %d characters", maxWidgetDescription)
	}
	if utf8.RuneCountInString(w.Icon) > maxWidgetIcon {
		return invalidf("icon must be at most %d characters", maxWidgetIcon)
	}
	tags := make([]string, 0, len(w.Tags))
	for _, t := range w.Tags {
		if t = strings.TrimSpace(t); t == "" {
			continue
		}
		if utf8.RuneCountInString(t) > maxWidgetTag {
			return invalidf("a tag must be at most %d characters", maxWidgetTag)
		}
		tags = append(tags, t)
	}
	if len(tags) > maxWidgetTags {
		return invalidf("a widget has at most %d tags", maxWidgetTags)
	}
	w.Tags = tags
	return nil
}

// maxWidgetName is the longest widget name.
const maxWidgetName = 60

// checkSchema validates a list of prop schemas. depth is 0 for a widget's
// fields and 1 inside an items list, which cannot nest further.
func checkSchema(ps []Prop, what string, limit, depth int) error {
	if len(ps) > limit {
		return invalidf("at most %d %ss", limit, what)
	}
	seen := map[string]bool{}
	for i := range ps {
		p := &ps[i]
		if len(p.Key) > maxFieldKey || !fieldKeyRE.MatchString(p.Key) {
			return invalidf("%s key %q must start with a lowercase letter and use only letters, digits and _ (at most %d characters)", what, p.Key, maxFieldKey)
		}
		if seen[p.Key] {
			return invalidf("%s key %q is used twice", what, p.Key)
		}
		seen[p.Key] = true
		if err := checkPropSchema(p, depth); err != nil {
			return invalidf("%s %q: %s", what, p.Key, strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": "))
		}
	}
	return nil
}

func checkPropSchema(p *Prop, depth int) error {
	if p.Label = strings.TrimSpace(p.Label); p.Label == "" {
		p.Label = p.Key
	}
	for _, c := range []struct {
		name string
		v    string
		max  int
	}{{"label", p.Label, maxFieldLabel}, {"help", p.Help, maxFieldHelp}, {"section", p.Section, maxFieldSection}, {"unit", p.Unit, maxFieldUnit}} {
		if utf8.RuneCountInString(c.v) > c.max {
			return invalidf("%s must be at most %d characters", c.name, c.max)
		}
	}
	if p.MaxLen < 0 || p.MaxLen > maxPropMaxLen {
		return invalidf("max length must be 0 to %d", maxPropMaxLen)
	}
	switch p.Kind {
	case PropText, PropTextarea, PropColor, PropFont, PropEffects, PropFrame, PropFolder, PropBool:
		return nil
	case PropNumber:
		return checkNumberSchema(p)
	case PropEnum:
		return checkEnumSchema(p)
	case PropMedia:
		for _, a := range p.Accept {
			if a != MediaImage && a != MediaVideo && a != MediaAudio {
				return invalidf("media kind %q is not image, video or audio", a)
			}
		}
		return nil
	case PropItems:
		return checkItemsSchema(p, depth)
	}
	return invalidf("unknown kind %q", p.Kind)
}

func checkNumberSchema(p *Prop) error {
	for _, v := range []float64{p.Min, p.Max, p.Step} {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > maxBound {
			return invalidf("number bounds must be within ±%g", maxBound)
		}
	}
	// A maximum at or below the minimum means no upper limit (Prop.HasMax).
	if p.Step < 0 {
		return invalidf("step must not be negative")
	}
	return nil
}

func checkEnumSchema(p *Prop) error {
	if len(p.Options) == 0 || len(p.Options) > maxFieldOptions {
		return invalidf("a choice needs 1 to %d options", maxFieldOptions)
	}
	seen := map[string]bool{}
	for i := range p.Options {
		o := &p.Options[i]
		if o.Value == "" || len(o.Value) > maxWidgetKey || seen[o.Value] {
			return invalidf("option values must be unique and 1 to %d bytes", maxWidgetKey)
		}
		seen[o.Value] = true
		if o.Label = strings.TrimSpace(o.Label); o.Label == "" {
			o.Label = o.Value
		}
		if utf8.RuneCountInString(o.Label) > maxFieldLabel {
			return invalidf("option labels must be at most %d characters", maxFieldLabel)
		}
	}
	return nil
}

func checkItemsSchema(p *Prop, depth int) error {
	if depth > 0 {
		return invalidf("a list cannot hold another list")
	}
	if len(p.Item) == 0 {
		return invalidf("a list needs at least one item field")
	}
	if p.Min < 0 || p.Max < 1 || p.Max > maxItemsMax || p.Min > p.Max || p.Min != math.Trunc(p.Min) || p.Max != math.Trunc(p.Max) {
		return invalidf("a list holds a whole number of items, 0 to %d", maxItemsMax)
	}
	return checkSchema(p.Item, "item field", maxItemFields, depth+1)
}

// checkWidgetDefaults validates the defaults object against the fields and
// stores it compacted. Fields without a default start at their zero value.
func checkWidgetDefaults(w *Definition) error {
	raw := bytes.TrimSpace(w.Defaults)
	if len(raw) > MaxWidgetCode {
		return invalidf("defaults are larger than %d bytes", MaxWidgetCode)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return invalidf("defaults must be a JSON object keyed by field")
	}
	byKey := map[string]Prop{}
	for _, p := range w.Fields {
		byKey[p.Key] = p
	}
	for key, v := range m {
		p, ok := byKey[key]
		if !ok {
			return invalidf("defaults name %q, which is not a field", key)
		}
		if err := checkFieldValue(p, v); err != nil {
			return invalidf("default of %q: %s", key, err)
		}
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return invalidf("defaults: %v", err)
	}
	w.Defaults = buf.Bytes()
	return nil
}

// checkFieldValue checks one decoded value against its field: the JSON
// type first (checkProp is lenient about types), then the field's rules.
func checkFieldValue(p Prop, v any) error {
	if !typeFits(p.Kind, v) {
		return fmt.Errorf("%s is not a %s value", jsonType(v), p.Kind)
	}
	return checkProp(p, v)
}

func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "a boolean"
	case float64:
		return "a number"
	case string:
		return "a string"
	case []any:
		return "a list"
	}
	return "an object"
}

// typeFits reports whether the decoded JSON value has the type a prop of
// that kind holds.
func typeFits(k PropKind, v any) bool {
	switch k {
	case PropNumber:
		_, ok := v.(float64)
		return ok
	case PropBool:
		_, ok := v.(bool)
		return ok
	case PropItems:
		l, ok := v.([]any)
		if !ok {
			return false
		}
		return !slices.ContainsFunc(l, func(it any) bool { _, ok := it.(map[string]any); return !ok })
	case PropEffects, PropFrame:
		_, ok := v.(map[string]any)
		return ok
	}
	_, ok := v.(string)
	return ok
}

func checkWidgetData(data []string) error {
	if len(data) > MaxWidgetData {
		return invalidf("a widget reads at most %d data keys", MaxWidgetData)
	}
	seen := map[string]bool{}
	for _, d := range data {
		if len(d) > maxWidgetKey || !dataKeyRE.MatchString(d) {
			return invalidf("data key %q must be lowercase words joined by dots, optionally ending in .*", d)
		}
		if seen[d] {
			return invalidf("data key %q is listed twice", d)
		}
		seen[d] = true
	}
	return nil
}

func checkWidgetSample(sample map[string]json.RawMessage) error {
	total := 0
	for k, v := range sample {
		if k == "" || len(k) > maxWidgetKey {
			return invalidf("sample keys must be 1 to %d bytes", maxWidgetKey)
		}
		if !json.Valid(v) {
			return invalidf("sample %q is not valid JSON", k)
		}
		total += len(k) + len(v)
	}
	if total > MaxWidgetSample {
		return invalidf("sample data is larger than %d bytes", MaxWidgetSample)
	}
	return nil
}
