package csw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

var (
	fillKinds  = []string{"", FillSolid, FillLinear, FillRadial, FillMedia}
	decoLines  = []string{"", LineNone, LineUnderline, LineThrough, LineOverline}
	decoStyles = []string{"", LineSolid, LineWavy, LineDotted, LineDashed, LineDouble}
	fitKinds   = []string{"", FitNone, FitShrink}
	animKinds  = []string{
		"", AnimNone, AnimPulse, AnimFloat, AnimShake, AnimShine, AnimRainbow,
		AnimWave, AnimBounce, AnimJitter, AnimTypewriter,
	}
	blendModes = []string{
		"", "normal", "multiply", "screen", "overlay", "darken", "lighten", "color-dodge", "color-burn",
		"hard-light", "soft-light", "difference", "exclusion", "hue", "saturation", "color", "luminosity",
	}
)

// DecodeEffects decodes a JSON effects value strictly and checks it. An
// empty or null value is the zero Effects.
func DecodeEffects(raw json.RawMessage) (Effects, error) {
	var e Effects
	if len(raw) > MaxEffectsBytes {
		return Effects{}, invalidf("effects are larger than %d bytes", MaxEffectsBytes)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			return Effects{}, invalidf("effects: %v", err)
		}
		if dec.More() {
			return Effects{}, invalidf("effects: trailing data")
		}
	}
	e.Canon()
	if err := e.Validate(); err != nil {
		return Effects{}, err
	}
	return e, nil
}

// Canon replaces nil lists with empty ones, so the value always encodes as
// the same JSON shape.
func (e *Effects) Canon() {
	if e.Outlines == nil {
		e.Outlines = []Outline{}
	}
	if e.Shadows == nil {
		e.Shadows = []Shadow{}
	}
	if e.Fill.Stops == nil {
		e.Fill.Stops = []Stop{}
	}
}

// Clone returns a deep copy.
func (e Effects) Clone() Effects {
	e.Fill.Stops = slices.Clone(e.Fill.Stops)
	e.Outlines = slices.Clone(e.Outlines)
	e.Shadows = slices.Clone(e.Shadows)
	e.Canon()
	return e
}

// Validate reports the first rule the effects break. The error wraps
// ErrInvalid.
func (e Effects) Validate() error {
	checks := []func() error{
		e.Fill.validate,
		func() error { return validateOutlines(e.Outlines) },
		func() error { return validateShadows(e.Shadows) },
		e.Glow.validate, e.Extrude.validate, e.Highlight.validate,
		e.Decoration.validate, e.Layout.validate, e.Animation.validate,
		func() error { return oneOf("blend", e.Blend, blendModes) },
	}
	for _, c := range checks {
		if err := c(); err != nil {
			return err
		}
	}
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("csw: effects: %w", err)
	}
	if len(b) > MaxEffectsBytes {
		return invalidf("effects are larger than %d bytes", MaxEffectsBytes)
	}
	return nil
}

func oneOf(what, v string, set []string) error {
	if !slices.Contains(set, v) {
		return invalidf("%s %q is not supported", what, v)
	}
	return nil
}

func intIn(what string, v, lo, hi int) error {
	if v < lo || v > hi {
		return invalidf("%s must be %d to %d", what, lo, hi)
	}
	return nil
}

// effColor checks a color (literal or theme reference); optional allows "".
func effColor(what, v string, optional bool) error {
	if v == "" && optional {
		return nil
	}
	if !colorRE.MatchString(v) && !isThemeColor(v) {
		return invalidf("%s %q is not a #rrggbb, #rrggbbaa or theme color", what, v)
	}
	return nil
}

func (f Fill) validate() error {
	if err := oneOf("fill kind", f.Kind, fillKinds); err != nil {
		return err
	}
	if err := intIn("fill angle", f.Angle, -360, 360); err != nil {
		return err
	}
	if len(f.Stops) > MaxStops {
		return invalidf("a fill has at most %d color stops", MaxStops)
	}
	if (f.Kind == FillLinear || f.Kind == FillRadial) && len(f.Stops) < 2 {
		return invalidf("a gradient needs 2 to %d color stops", MaxStops)
	}
	for _, s := range f.Stops {
		if err := effColor("gradient color", s.Color, false); err != nil {
			return err
		}
		if err := intIn("gradient stop position", s.At, 0, 100); err != nil {
			return err
		}
	}
	if f.Kind == FillMedia && checkID(f.Media, prefixMedia) != nil {
		return invalidf("fill image %q is not a media item", f.Media)
	}
	if f.Media != "" && checkID(f.Media, prefixMedia) != nil {
		return invalidf("fill image %q is not a media item", f.Media)
	}
	return nil
}

func validateOutlines(os []Outline) error {
	if len(os) > MaxOutlines {
		return invalidf("text has at most %d outlines", MaxOutlines)
	}
	for _, o := range os {
		if err := intIn("outline width", o.Width, 1, 50); err != nil {
			return err
		}
		if err := effColor("outline color", o.Color, false); err != nil {
			return err
		}
	}
	return nil
}

func validateShadows(ss []Shadow) error {
	if len(ss) > MaxShadows {
		return invalidf("text has at most %d shadows", MaxShadows)
	}
	for _, s := range ss {
		if err := cmpErr(
			func() error { return intIn("shadow x offset", s.X, -200, 200) },
			func() error { return intIn("shadow y offset", s.Y, -200, 200) },
			func() error { return intIn("shadow blur", s.Blur, 0, 200) },
		); err != nil {
			return err
		}
		if err := effColor("shadow color", s.Color, false); err != nil {
			return err
		}
	}
	return nil
}

// cmpErr returns the first error the checks report.
func cmpErr(checks ...func() error) error {
	for _, c := range checks {
		if err := c(); err != nil {
			return err
		}
	}
	return nil
}

func (g Glow) validate() error {
	if err := intIn("glow size", g.Size, 0, 200); err != nil {
		return err
	}
	if err := intIn("glow strength", g.Strength, 0, 5); err != nil {
		return err
	}
	if g.Size > 0 && g.Strength < 1 {
		return invalidf("glow strength must be 1 to 5")
	}
	return effColor("glow color", g.Color, g.Size == 0)
}

func (x Extrude) validate() error {
	if err := intIn("extrude depth", x.Depth, 0, 60); err != nil {
		return err
	}
	if err := intIn("extrude angle", x.Angle, -360, 360); err != nil {
		return err
	}
	return effColor("extrude color", x.Color, x.Depth == 0)
}

func (h Highlight) validate() error {
	for _, c := range []struct {
		what string
		v    int
	}{{"highlight horizontal padding", h.PadX}, {"highlight vertical padding", h.PadY}, {"highlight radius", h.Radius}} {
		if err := intIn(c.what, c.v, 0, 200); err != nil {
			return err
		}
	}
	return effColor("highlight color", h.Color, true)
}

func (d Decoration) validate() error {
	if err := oneOf("decoration line", d.Line, decoLines); err != nil {
		return err
	}
	if err := oneOf("decoration style", d.Style, decoStyles); err != nil {
		return err
	}
	if err := intIn("decoration thickness", d.Thickness, 0, 50); err != nil {
		return err
	}
	if err := intIn("decoration offset", d.Offset, -50, 50); err != nil {
		return err
	}
	return effColor("decoration color", d.Color, true)
}

func (l TextLayout) validate() error {
	if err := oneOf("fit", l.Fit, fitKinds); err != nil {
		return err
	}
	if l.MinSize != 0 {
		if err := intIn("minimum size", l.MinSize, 4, 1000); err != nil {
			return err
		}
	}
	if err := intIn("skew", l.Skew, -45, 45); err != nil {
		return err
	}
	if l.Stretch != 0 {
		return intIn("stretch", l.Stretch, 50, 200)
	}
	return nil
}

func (a Animation) validate() error {
	if err := oneOf("animation", a.Kind, animKinds); err != nil {
		return err
	}
	if math.IsNaN(a.Speed) || math.IsInf(a.Speed, 0) || (a.Speed != 0 && (a.Speed < 0.2 || a.Speed > 20)) {
		return invalidf("animation speed must be 0.2 to 20 seconds")
	}
	return intIn("animation intensity", a.Intensity, 0, 10)
}
