package csw

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

const testMedia = "media-00000000000000000000000001"

func stops(n int) []Stop {
	out := make([]Stop, n)
	for i := range out {
		out[i] = Stop{Color: "#ff0000", At: i * 10}
	}
	return out
}

func TestEffectsValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mut  func(*Effects)
		want string // empty: valid
	}{
		{"zero", func(*Effects) {}, ""},
		{"solid fill", func(e *Effects) { e.Fill.Kind = FillSolid }, ""},
		{"unknown fill", func(e *Effects) { e.Fill.Kind = "plaid" }, "fill kind"},
		{"linear ok", func(e *Effects) { e.Fill = Fill{Kind: FillLinear, Angle: 90, Stops: stops(2)} }, ""},
		{"radial ok", func(e *Effects) { e.Fill = Fill{Kind: FillRadial, Stops: stops(MaxStops)} }, ""},
		{"gradient one stop", func(e *Effects) { e.Fill = Fill{Kind: FillLinear, Stops: stops(1)} }, "color stops"},
		{"gradient no stops", func(e *Effects) { e.Fill = Fill{Kind: FillRadial} }, "color stops"},
		{"too many stops", func(e *Effects) { e.Fill = Fill{Kind: FillLinear, Stops: stops(MaxStops + 1)} }, "at most"},
		{"angle low", func(e *Effects) { e.Fill.Angle = -361 }, "fill angle"},
		{"angle high", func(e *Effects) { e.Fill.Angle = 361 }, "fill angle"},
		{"stop color", func(e *Effects) { e.Fill = Fill{Kind: FillLinear, Stops: []Stop{{Color: "red"}, {Color: "#000000"}}} }, "color"},
		{"stop empty color", func(e *Effects) { e.Fill = Fill{Kind: FillLinear, Stops: []Stop{{Color: ""}, {Color: "#000000"}}} }, "color"},
		{"stop at high", func(e *Effects) {
			e.Fill = Fill{Kind: FillLinear, Stops: []Stop{{Color: "#000000", At: 101}, {Color: "#000000"}}}
		}, "position"},
		{"stop at low", func(e *Effects) {
			e.Fill = Fill{Kind: FillLinear, Stops: []Stop{{Color: "#000000", At: -1}, {Color: "#000000"}}}
		}, "position"},
		{"media ok", func(e *Effects) { e.Fill = Fill{Kind: FillMedia, Media: testMedia} }, ""},
		{"media missing", func(e *Effects) { e.Fill = Fill{Kind: FillMedia} }, "media item"},
		{"media bad", func(e *Effects) { e.Fill = Fill{Kind: FillMedia, Media: "nope"} }, "media item"},
		{"media kept on solid", func(e *Effects) { e.Fill = Fill{Kind: FillSolid, Media: testMedia} }, ""},
		{"bad media on solid", func(e *Effects) { e.Fill = Fill{Kind: FillSolid, Media: "x"} }, "media item"},
		{"outline ok", func(e *Effects) {
			e.Outlines = []Outline{{Width: 50, Color: "#00000080"}, {Width: 1, Color: "#fff000"}}
		}, ""},
		{"outlines many", func(e *Effects) { e.Outlines = make([]Outline, MaxOutlines+1) }, "at most"},
		{"outline zero width", func(e *Effects) { e.Outlines = []Outline{{Width: 0, Color: "#000000"}} }, "outline width"},
		{"outline wide", func(e *Effects) { e.Outlines = []Outline{{Width: 51, Color: "#000000"}} }, "outline width"},
		{"outline color", func(e *Effects) { e.Outlines = []Outline{{Width: 2, Color: ""}} }, "outline color"},
		{"shadow ok", func(e *Effects) { e.Shadows = []Shadow{{X: -200, Y: 200, Blur: 200, Color: "#000000"}} }, ""},
		{"shadows many", func(e *Effects) {
			e.Shadows = make([]Shadow, MaxShadows+1)
			for i := range e.Shadows {
				e.Shadows[i].Color = "#000000"
			}
		}, "at most"},
		{"shadow x", func(e *Effects) { e.Shadows = []Shadow{{X: 201, Color: "#000000"}} }, "shadow x"},
		{"shadow y", func(e *Effects) { e.Shadows = []Shadow{{Y: -201, Color: "#000000"}} }, "shadow y"},
		{"shadow blur", func(e *Effects) { e.Shadows = []Shadow{{Blur: 201, Color: "#000000"}} }, "shadow blur"},
		{"shadow blur negative", func(e *Effects) { e.Shadows = []Shadow{{Blur: -1, Color: "#000000"}} }, "shadow blur"},
		{"shadow color", func(e *Effects) { e.Shadows = []Shadow{{Color: "black"}} }, "shadow color"},
		{"glow ok", func(e *Effects) { e.Glow = Glow{Size: 200, Strength: 5, Color: "#ffffff"} }, ""},
		{"glow big", func(e *Effects) { e.Glow = Glow{Size: 201, Strength: 1, Color: "#ffffff"} }, "glow size"},
		{"glow strong", func(e *Effects) { e.Glow = Glow{Size: 4, Strength: 6, Color: "#ffffff"} }, "glow strength"},
		{"glow weak", func(e *Effects) { e.Glow = Glow{Size: 4, Strength: 0, Color: "#ffffff"} }, "glow strength"},
		{"glow negative", func(e *Effects) { e.Glow.Strength = -1 }, "glow strength"},
		{"glow color", func(e *Effects) { e.Glow = Glow{Size: 4, Strength: 1} }, "glow color"},
		{"glow off keeps color", func(e *Effects) { e.Glow = Glow{Color: "#ffffff"} }, ""},
		{"glow off bad color", func(e *Effects) { e.Glow = Glow{Color: "x"} }, "glow color"},
		{"extrude ok", func(e *Effects) { e.Extrude = Extrude{Depth: 60, Angle: 135, Color: "#000000"} }, ""},
		{"extrude deep", func(e *Effects) { e.Extrude = Extrude{Depth: 61, Color: "#000000"} }, "extrude depth"},
		{"extrude angle", func(e *Effects) { e.Extrude = Extrude{Depth: 1, Angle: 400, Color: "#000000"} }, "extrude angle"},
		{"extrude color", func(e *Effects) { e.Extrude = Extrude{Depth: 1} }, "extrude color"},
		{"highlight ok", func(e *Effects) { e.Highlight = Highlight{Color: "#ffff00", PadX: 200, PadY: 200, Radius: 200} }, ""},
		{"highlight padx", func(e *Effects) { e.Highlight.PadX = 201 }, "horizontal"},
		{"highlight pady", func(e *Effects) { e.Highlight.PadY = -1 }, "vertical"},
		{"highlight radius", func(e *Effects) { e.Highlight.Radius = 201 }, "radius"},
		{"highlight color", func(e *Effects) { e.Highlight.Color = "yellow" }, "highlight color"},
		{"decoration ok", func(e *Effects) {
			e.Decoration = Decoration{Line: LineUnderline, Style: LineWavy, Color: "#ff0000", Thickness: 50, Offset: -50}
		}, ""},
		{"decoration line", func(e *Effects) { e.Decoration.Line = "strike" }, "decoration line"},
		{"decoration style", func(e *Effects) { e.Decoration.Style = "zigzag" }, "decoration style"},
		{"decoration thick", func(e *Effects) { e.Decoration.Thickness = 51 }, "thickness"},
		{"decoration offset", func(e *Effects) { e.Decoration.Offset = 51 }, "offset"},
		{"decoration color", func(e *Effects) { e.Decoration.Color = "red" }, "decoration color"},
		{"blend ok", func(e *Effects) { e.Blend = "soft-light" }, ""},
		{"blend normal", func(e *Effects) { e.Blend = "normal" }, ""},
		{"blend bad", func(e *Effects) { e.Blend = "plus-lighter" }, "blend"},
		{"layout ok", func(e *Effects) {
			e.Layout = TextLayout{Fit: FitShrink, MinSize: 4, Skew: -45, Stretch: 200, Ellipsis: true, Balance: true, Vertical: true}
		}, ""},
		{"fit bad", func(e *Effects) { e.Layout.Fit = "grow" }, "fit"},
		{"minsize low", func(e *Effects) { e.Layout.MinSize = 3 }, "minimum size"},
		{"minsize high", func(e *Effects) { e.Layout.MinSize = 1001 }, "minimum size"},
		{"skew", func(e *Effects) { e.Layout.Skew = 46 }, "skew"},
		{"stretch low", func(e *Effects) { e.Layout.Stretch = 49 }, "stretch"},
		{"stretch high", func(e *Effects) { e.Layout.Stretch = 201 }, "stretch"},
		{"animation ok", func(e *Effects) { e.Animation = Animation{Kind: AnimTypewriter, Speed: 20, Intensity: 10} }, ""},
		{"animation slow ok", func(e *Effects) { e.Animation = Animation{Kind: AnimWave, Speed: 0.2, Intensity: 1} }, ""},
		{"animation kind", func(e *Effects) { e.Animation.Kind = "spin" }, "animation"},
		{"animation fast", func(e *Effects) { e.Animation.Speed = 0.1 }, "speed"},
		{"animation slow", func(e *Effects) { e.Animation.Speed = 21 }, "speed"},
		{"animation nan", func(e *Effects) { e.Animation.Speed = math.NaN() }, "speed"},
		{"animation inf", func(e *Effects) { e.Animation.Speed = math.Inf(1) }, "speed"},
		{"intensity", func(e *Effects) { e.Animation.Intensity = 11 }, "intensity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var e Effects
			tt.mut(&e)
			e.Canon()
			err := e.Validate()
			switch {
			case tt.want == "" && err != nil:
				t.Fatalf("err = %v, want valid", err)
			case tt.want != "" && (!errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tt.want)):
				t.Fatalf("err = %v, want invalid containing %q", err, tt.want)
			}
		})
	}
}

func TestDecodeEffects(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "null", " null ", "{}"} {
		e, err := DecodeEffects(json.RawMessage(in))
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		b, _ := json.Marshal(e)
		if !strings.Contains(string(b), `"outlines":[]`) || !strings.Contains(string(b), `"shadows":[]`) || !strings.Contains(string(b), `"stops":[]`) {
			t.Errorf("%q: encoded %s, want empty lists", in, b)
		}
	}
	bad := map[string]string{
		"unknown":       `{"sparkle":1}`,
		"nested":        `{"glow":{"size":1,"strength":1,"color":"#fff000","x":1}}`,
		"wrong type":    `{"outlines":"x"}`,
		"trailing":      `{} {}`,
		"not json":      `{`,
		"invalid value": `{"glow":{"size":999}}`,
		"too big":       `{"blend":"normal"}` + strings.Repeat(" ", MaxEffectsBytes),
	}
	for name, in := range bad {
		if _, err := DecodeEffects(json.RawMessage(in)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want invalid", name, err)
		}
	}
}

func TestEffectsClone(t *testing.T) {
	t.Parallel()
	e := Effects{Outlines: []Outline{{Width: 1, Color: "#000000"}}}
	c := e.Clone()
	c.Outlines[0].Width = 9
	if e.Outlines[0].Width != 1 || c.Shadows == nil || c.Fill.Stops == nil {
		t.Fatalf("clone shares or is not canonical: %+v %+v", e, c)
	}
}
