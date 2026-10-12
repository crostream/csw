package csw

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

var (
	shapeKinds = []string{
		"", ShapeRect, ShapeCut, ShapeNotch, ShapePill, ShapeSlant, ShapeTicket, ShapeHexagon, ShapeBubble,
		ShapeEllipse, ShapeTriangle, ShapeStar, ShapePolygon, ShapeHeart, ShapeArrow,
	}
	edgeKinds     = []string{"", EdgeTop, EdgeRight, EdgeBottom, EdgeLeft}
	frameFills    = []string{"", FillNone, FillSolid, FillLinear, FillRadial, FillMedia}
	patternKinds  = []string{"", PatternNone, PatternStripes, PatternDots, PatternGrid, PatternChecker, PatternScanlines, PatternCarbon}
	borderStyles  = []string{"", BorderSolid, BorderDashed, BorderDotted, BorderDouble}
	textureKinds  = []string{"", TextureNone, TextureNoise, TextureGrain}
	repeatModes   = []string{"", RepeatStretch, RepeatRepeat, RepeatRound, RepeatSpace}
	frameAnimKind = []string{"", FrameAnimNone, FrameAnimRunning, FrameAnimAnts, FrameAnimPulse, FrameAnimShine}
)

// DecodeFrame decodes a JSON frame strictly and checks it. An empty or null
// value is the zero FrameStyle.
func DecodeFrame(raw json.RawMessage) (FrameStyle, error) {
	var f FrameStyle
	if len(raw) > MaxFrameBytes {
		return FrameStyle{}, invalidf("frame is larger than %d bytes", MaxFrameBytes)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&f); err != nil {
			return FrameStyle{}, invalidf("frame: %v", err)
		}
		if dec.More() {
			return FrameStyle{}, invalidf("frame: trailing data")
		}
	}
	f.Canon()
	if err := f.Validate(); err != nil {
		return FrameStyle{}, err
	}
	return f, nil
}

// Canon replaces nil lists with empty ones, so the value always encodes as
// the same JSON shape.
func (f *FrameStyle) Canon() {
	if f.Rings == nil {
		f.Rings = []Ring{}
	}
	if f.Shadows == nil {
		f.Shadows = []BoxShadow{}
	}
	if f.Fill.Stops == nil {
		f.Fill.Stops = []Stop{}
	}
	if f.Border.Stops == nil {
		f.Border.Stops = []Stop{}
	}
}

// Clone returns a deep copy.
func (f FrameStyle) Clone() FrameStyle {
	f.Rings = slices.Clone(f.Rings)
	f.Shadows = slices.Clone(f.Shadows)
	f.Fill.Stops = slices.Clone(f.Fill.Stops)
	f.Border.Stops = slices.Clone(f.Border.Stops)
	f.Canon()
	return f
}

// Validate reports the first rule the frame breaks. The error wraps
// ErrInvalid.
func (f FrameStyle) Validate() error {
	if err := cmpErr(
		f.Shape.validate, f.Fill.validate, f.Border.validate,
		func() error { return validateRings(f.Rings) },
		f.Brackets.validate,
		func() error { return validateBoxShadows(f.Shadows) },
		f.Glow.validate, f.Bevel.validate, f.Texture.validate, f.Image.validate, f.Animation.validate,
		func() error { return intIn("padding", f.Padding, 0, MaxFrameLength) },
	); err != nil {
		return err
	}
	b, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("csw: frame: %w", err)
	}
	if len(b) > MaxFrameBytes {
		return invalidf("frame is larger than %d bytes", MaxFrameBytes)
	}
	return nil
}

// length is a named value that must be 0 to MaxFrameLength.
type length struct {
	what string
	v    int
}

func lengths(ls ...length) error {
	for _, l := range ls {
		if err := intIn(l.what, l.v, 0, MaxFrameLength); err != nil {
			return err
		}
	}
	return nil
}

func (s Shape) validate() error {
	if err := oneOf("shape", s.Kind, shapeKinds); err != nil {
		return err
	}
	for _, r := range s.Radius {
		if err := intIn("corner radius", r, 0, MaxFrameLength); err != nil {
			return err
		}
	}
	if err := intIn("slant", s.Slant, -45, 45); err != nil {
		return err
	}
	if err := shapePoints(s); err != nil {
		return err
	}
	return cmpErr(
		func() error { return oneOf("notch edge", s.Notch.Edge, edgeKinds) },
		func() error { return intIn("notch position", s.Notch.At, 0, 100) },
		func() error { return intIn("notch width", s.Notch.Width, 0, MaxFrameLength) },
		func() error { return intIn("notch depth", s.Notch.Depth, -MaxFrameLength, MaxFrameLength) },
		func() error { return oneOf("pointer edge", s.Pointer.Edge, edgeKinds) },
		func() error { return intIn("pointer position", s.Pointer.At, 0, 100) },
		func() error { return intIn("pointer size", s.Pointer.Size, 0, MaxFrameLength) },
	)
}

// shapePoints checks the star and polygon settings of a shape.
func shapePoints(s Shape) error {
	if s.Points != 0 {
		if err := intIn("shape points", s.Points, 3, 12); err != nil {
			return err
		}
	}
	if s.Inner != 0 {
		return intIn("shape inner radius", s.Inner, 10, 90)
	}
	return nil
}

// validateStops checks a gradient's stops: none, or 2 to MaxStops colors
// at positions 0 to 100 (needed true: at least two).
func validateStops(what string, stops []Stop, needed bool) error {
	if len(stops) > MaxStops {
		return invalidf("%s has at most %d color stops", what, MaxStops)
	}
	if (needed || len(stops) > 0) && len(stops) < 2 {
		return invalidf("%s needs 2 to %d color stops", what, MaxStops)
	}
	for _, s := range stops {
		if err := effColor(what+" color", s.Color, false); err != nil {
			return err
		}
		if err := intIn(what+" stop position", s.At, 0, 100); err != nil {
			return err
		}
	}
	return nil
}

func (f FrameFill) validate() error {
	if err := oneOf("fill kind", f.Kind, frameFills); err != nil {
		return err
	}
	if err := intIn("fill angle", f.Angle, -360, 360); err != nil {
		return err
	}
	gradient := f.Kind == FillLinear || f.Kind == FillRadial
	if err := validateStops("fill", f.Stops, gradient); err != nil {
		return err
	}
	if err := effColor("fill color", f.Color, f.Kind != FillSolid); err != nil {
		return err
	}
	if (f.Kind == FillMedia || f.Media != "") && checkID(f.Media, prefixMedia) != nil {
		return invalidf("fill image %q is not a media item", f.Media)
	}
	return f.Pattern.validate()
}

func (p Pattern) validate() error {
	if err := oneOf("pattern", p.Kind, patternKinds); err != nil {
		return err
	}
	if p.Size != 0 {
		if err := intIn("pattern size", p.Size, 2, 400); err != nil {
			return err
		}
	}
	if err := intIn("pattern angle", p.Angle, -360, 360); err != nil {
		return err
	}
	return effColor("pattern color", p.Color, p.Kind == "" || p.Kind == PatternNone)
}

func (b Border) validate() error {
	if err := oneOf("border style", b.Style, borderStyles); err != nil {
		return err
	}
	if err := cmpErr(
		func() error { return intIn("border width", b.Width, 0, MaxFrameLength) },
		func() error { return intIn("border angle", b.Angle, -360, 360) },
		func() error { return intIn("border inset", b.Inset, -MaxFrameLength, MaxFrameLength) },
		func() error { return validateStops("border", b.Stops, false) },
	); err != nil {
		return err
	}
	return effColor("border color", b.Color, b.Width == 0 || len(b.Stops) >= 2)
}

func validateRings(rs []Ring) error {
	if len(rs) > MaxRings {
		return invalidf("a frame has at most %d rings", MaxRings)
	}
	for _, r := range rs {
		if err := lengths(length{"ring width", r.Width}, length{"ring gap", r.Gap}); err != nil {
			return err
		}
		if err := effColor("ring color", r.Color, false); err != nil {
			return err
		}
	}
	return nil
}

func (b Brackets) validate() error {
	if err := lengths(length{"bracket length", b.Length}, length{"bracket width", b.Width}); err != nil {
		return err
	}
	if err := intIn("bracket inset", b.Inset, -MaxFrameLength, MaxFrameLength); err != nil {
		return err
	}
	return effColor("bracket color", b.Color, b.Length == 0)
}

func validateBoxShadows(ss []BoxShadow) error {
	if len(ss) > MaxBoxShadows {
		return invalidf("a frame has at most %d shadows", MaxBoxShadows)
	}
	for _, s := range ss {
		if err := cmpErr(
			func() error { return intIn("shadow x offset", s.X, -MaxFrameLength, MaxFrameLength) },
			func() error { return intIn("shadow y offset", s.Y, -MaxFrameLength, MaxFrameLength) },
			func() error { return intIn("shadow blur", s.Blur, 0, MaxFrameLength) },
			func() error { return intIn("shadow spread", s.Spread, -MaxFrameLength, MaxFrameLength) },
			func() error { return effColor("shadow color", s.Color, false) },
		); err != nil {
			return err
		}
	}
	return nil
}

func (b Bevel) validate() error {
	if err := intIn("bevel size", b.Size, 0, MaxFrameLength); err != nil {
		return err
	}
	if err := effColor("bevel light color", b.Light, b.Size == 0); err != nil {
		return err
	}
	return effColor("bevel dark color", b.Dark, b.Size == 0)
}

func (t Texture) validate() error {
	if err := oneOf("texture", t.Kind, textureKinds); err != nil {
		return err
	}
	return intIn("texture opacity", t.Opacity, 0, 100)
}

// validate checks the 9-slice. The media may be empty while the slices are
// set (the streamer cleared the image); the box then draws from its shape.
func (n NineSlice) validate() error {
	if n.Media != "" && checkID(n.Media, prefixMedia) != nil {
		return invalidf("frame image %q is not a media item", n.Media)
	}
	for _, v := range n.Slice {
		if err := intIn("image slice", v, 0, MaxFrameLength); err != nil {
			return err
		}
	}
	for _, v := range n.Width {
		if err := intIn("image border width", v, 0, MaxFrameLength); err != nil {
			return err
		}
	}
	return oneOf("image repeat", n.Repeat, repeatModes)
}

func (a FrameAnimation) validate() error {
	if err := oneOf("frame animation", a.Kind, frameAnimKind); err != nil {
		return err
	}
	if math.IsNaN(a.Speed) || math.IsInf(a.Speed, 0) || (a.Speed != 0 && (a.Speed < 0.2 || a.Speed > 20)) {
		return invalidf("frame animation speed must be 0.2 to 20 seconds")
	}
	return effColor("frame animation color", a.Color, true)
}

// checkFrameProp validates a decoded frame value of a normalized props
// object, which Normalize has already decoded strictly.
func checkFrameProp(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err = DecodeFrame(b); err != nil {
		// The caller adds the ErrInvalid prefix itself.
		return errors.New(strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": "))
	}
	return nil
}
