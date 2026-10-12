package csw

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
)

// PropKind says how the editor's inspector edits a prop and how Normalize
// checks it.
type PropKind string

// Prop kinds.
const (
	PropText     PropKind = "text"     // one line of text
	PropTextarea PropKind = "textarea" // several lines of text
	PropNumber   PropKind = "number"   // a number within Min and Max
	PropBool     PropKind = "bool"     // a switch
	PropColor    PropKind = "color"    // "#rrggbb", "#rrggbbaa" or a theme color ("theme:accent")
	PropEnum     PropKind = "enum"     // one of Options
	PropMedia    PropKind = "media"    // a media library item ID, or ""
	PropFont     PropKind = "font"     // a CSS font family list, or a theme font ("theme:body")
	PropEffects  PropKind = "effects"  // a text's Effects; the field is an Effects
	PropFrame    PropKind = "frame"    // a box's FrameStyle; the field is a FrameStyle
	PropItems    PropKind = "items"    // a list of objects shaped by Item; Min and Max bound the count
	PropFolder   PropKind = "folder"   // a media library folder ID ("folder-…"), or ""
)

// defaultMaxLen caps text props without their own MaxLen.
const defaultMaxLen = 10_000

// Media kinds a PropMedia accepts, as the library reports them.
const (
	MediaImage = "image"
	MediaVideo = "video"
	MediaAudio = "audio"
)

// Choice is one option of a PropEnum.
type Choice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Prop describes one setting of a widget. Key is the props struct's JSON
// field name.
type Prop struct {
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Help  string   `json:"help,omitempty"`
	Kind  PropKind `json:"kind"`
	// Section groups props under a heading in the inspector, e.g. "Outline".
	Section string `json:"section,omitempty"`
	// Min, Max, Step and Unit apply to PropNumber. A Max at or below Min
	// means no upper limit (see HasMax); built-in widgets always set one.
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Step float64 `json:"step,omitempty"`
	Unit string  `json:"unit,omitempty"`
	// Options lists the values of a PropEnum.
	Options []Choice `json:"options,omitempty"`
	// Accept lists the media kinds a PropMedia takes.
	Accept []string `json:"accept,omitempty"`
	// MaxLen caps text props (default 10,000 bytes).
	MaxLen int `json:"maxLen,omitempty"`
	// Optional lets a PropColor be empty (no color); the inspector offers a
	// clear button.
	Optional bool `json:"optional,omitempty"`
	// Item describes the fields of each object of a PropItems list. The
	// props struct's field is a slice of a struct with exactly these JSON
	// fields; Min and Max bound the number of items.
	Item []Prop `json:"item,omitempty"`
}

// Size is a width and height in canvas pixels.
type Size struct {
	W int `json:"w"`
	H int `json:"h"`
}

// colorRE matches "#rrggbb" and "#rrggbbaa".
var colorRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$`)

// HasMax reports whether a PropNumber has an upper limit: a Max at or below
// Min means none, as the widget editor shows it.
func (p Prop) HasMax() bool { return p.Max > p.Min }

func checkProp(p Prop, v any) error {
	switch p.Kind {
	case PropNumber:
		n, _ := v.(float64)
		switch {
		case math.IsNaN(n), p.HasMax() && (n < p.Min || n > p.Max):
			return fmt.Errorf("%v is outside %v to %v", v, p.Min, p.Max)
		case n < p.Min:
			return fmt.Errorf("%v is below the minimum %v", v, p.Min)
		}
		return nil
	case PropBool:
		return nil
	case PropEffects:
		return checkEffectsProp(v)
	case PropFrame:
		return checkFrameProp(v)
	case PropItems:
		return checkItems(p, v)
	}
	s, _ := v.(string)
	maxLen := p.MaxLen
	if maxLen == 0 {
		maxLen = defaultMaxLen
	}
	if len(s) > maxLen {
		return fmt.Errorf("longer than %d bytes", maxLen)
	}
	return checkStringProp(p, s)
}

// checkItems validates a PropItems list: its length, then every item's
// fields by the rules of their own kinds.
func checkItems(p Prop, v any) error {
	list, _ := v.([]any)
	if n := float64(len(list)); n < p.Min || n > p.Max {
		return fmt.Errorf("%d items is outside %v to %v", len(list), p.Min, p.Max)
	}
	for i, it := range list {
		m, _ := it.(map[string]any)
		for _, ip := range p.Item {
			if err := checkProp(ip, m[ip.Key]); err != nil {
				return fmt.Errorf("item %d %s: %w", i+1, ip.Key, err)
			}
		}
	}
	return nil
}

// checkStringProp checks the value of a prop that holds a string.
func checkStringProp(p Prop, s string) error {
	switch p.Kind {
	case PropColor:
		if (!p.Optional || s != "") && !colorRE.MatchString(s) && !isThemeColor(s) {
			return fmt.Errorf("%q is not a #rrggbb or #rrggbbaa color", s)
		}
	case PropEnum:
		if !slices.ContainsFunc(p.Options, func(c Choice) bool { return c.Value == s }) {
			return fmt.Errorf("%q is not one of the options", s)
		}
	case PropMedia:
		if s != "" && checkID(s, prefixMedia) != nil {
			return fmt.Errorf("%q is not a media item", s)
		}
	case PropFolder:
		if s != "" && checkID(s, prefixFolder) != nil {
			return fmt.Errorf("%q is not a media folder", s)
		}
	case PropFont:
		if !validFontName(s) {
			return fmt.Errorf("%q is not a font name", s)
		}
	}
	return nil
}

// checkEffectsProp validates a decoded effects value of a normalized props
// object, which Normalize has already decoded strictly.
func checkEffectsProp(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err = DecodeEffects(b); err != nil {
		// The caller adds the ErrInvalid prefix itself.
		return errors.New(strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": "))
	}
	return nil
}
