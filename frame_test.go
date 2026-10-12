package csw

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func fillShadows(ss []BoxShadow) {
	for i := range ss {
		ss[i].Color = "#00000080"
	}
}

func TestFrameValidate(t *testing.T) {
	t.Parallel()
	gradient := func(n int) []Stop {
		out := make([]Stop, n)
		for i := range out {
			out[i] = Stop{Color: "#112233", At: i * 10}
		}
		return out
	}
	tests := []struct {
		name string
		mut  func(*FrameStyle)
		want string // empty: valid
	}{
		{"zero", func(*FrameStyle) {}, ""},
		{"shape unknown", func(f *FrameStyle) { f.Shape.Kind = "blob" }, "shape"},
		{"shape every kind", func(f *FrameStyle) { f.Shape.Kind = ShapeHexagon }, ""},
		{"radius max", func(f *FrameStyle) { f.Shape.Radius = [4]int{MaxFrameLength, 0, 1, 2} }, ""},
		{"radius negative", func(f *FrameStyle) { f.Shape.Radius[2] = -1 }, "corner radius"},
		{"radius high", func(f *FrameStyle) { f.Shape.Radius[3] = MaxFrameLength + 1 }, "corner radius"},
		{"shape new kinds", func(f *FrameStyle) { f.Shape = Shape{Kind: ShapeStar, Points: 12, Inner: 90} }, ""},
		{"shape polygon", func(f *FrameStyle) { f.Shape = Shape{Kind: ShapePolygon, Points: 3} }, ""},
		{"shape heart", func(f *FrameStyle) { f.Shape.Kind = ShapeHeart }, ""},
		{"shape arrow", func(f *FrameStyle) { f.Shape.Kind = ShapeArrow }, ""},
		{"shape ellipse", func(f *FrameStyle) { f.Shape.Kind = ShapeEllipse }, ""},
		{"shape triangle", func(f *FrameStyle) { f.Shape.Kind = ShapeTriangle }, ""},
		{"points low", func(f *FrameStyle) { f.Shape.Points = 2 }, "shape points"},
		{"points high", func(f *FrameStyle) { f.Shape.Points = 13 }, "shape points"},
		{"inner low", func(f *FrameStyle) { f.Shape.Inner = 9 }, "shape inner"},
		{"inner high", func(f *FrameStyle) { f.Shape.Inner = 91 }, "shape inner"},
		{"slant ok", func(f *FrameStyle) { f.Shape.Slant = -45 }, ""},
		{"slant high", func(f *FrameStyle) { f.Shape.Slant = 46 }, "slant"},
		{"notch edge", func(f *FrameStyle) { f.Shape.Notch.Edge = "middle" }, "notch edge"},
		{"notch at", func(f *FrameStyle) { f.Shape.Notch.At = 101 }, "notch position"},
		{"notch width", func(f *FrameStyle) { f.Shape.Notch.Width = -1 }, "notch width"},
		{"notch depth negative ok", func(f *FrameStyle) { f.Shape.Notch.Depth = -MaxFrameLength }, ""},
		{"notch depth high", func(f *FrameStyle) { f.Shape.Notch.Depth = MaxFrameLength + 1 }, "notch depth"},
		{"pointer edge", func(f *FrameStyle) { f.Shape.Pointer.Edge = "up" }, "pointer edge"},
		{"pointer at", func(f *FrameStyle) { f.Shape.Pointer.At = -1 }, "pointer position"},
		{"pointer size", func(f *FrameStyle) { f.Shape.Pointer.Size = MaxFrameLength + 1 }, "pointer size"},

		{"fill none", func(f *FrameStyle) { f.Fill.Kind = FillNone }, ""},
		{"fill unknown", func(f *FrameStyle) { f.Fill.Kind = "plaid" }, "fill kind"},
		{"fill solid", func(f *FrameStyle) { f.Fill = FrameFill{Kind: FillSolid, Color: "#aabbcc"} }, ""},
		{"fill solid theme", func(f *FrameStyle) { f.Fill = FrameFill{Kind: FillSolid, Color: "theme:panel"} }, ""},
		{"fill solid no color", func(f *FrameStyle) { f.Fill.Kind = FillSolid }, "fill color"},
		{"fill solid bad theme", func(f *FrameStyle) { f.Fill = FrameFill{Kind: FillSolid, Color: "theme:nope"} }, "fill color"},
		{"fill angle", func(f *FrameStyle) { f.Fill.Angle = 361 }, "fill angle"},
		{"fill linear", func(f *FrameStyle) { f.Fill = FrameFill{Kind: FillLinear, Stops: gradient(2)} }, ""},
		{"fill linear one stop", func(f *FrameStyle) { f.Fill = FrameFill{Kind: FillLinear, Stops: gradient(1)} }, "color stops"},
		{"fill radial none", func(f *FrameStyle) { f.Fill.Kind = FillRadial }, "color stops"},
		{"fill too many stops", func(f *FrameStyle) { f.Fill = FrameFill{Kind: FillLinear, Stops: gradient(MaxStops + 1)} }, "at most"},
		{"fill stop theme", func(f *FrameStyle) {
			f.Fill = FrameFill{Kind: FillLinear, Stops: []Stop{{Color: "theme:primary"}, {Color: "theme:accent", At: 100}}}
		}, ""},
		{"fill stop color", func(f *FrameStyle) {
			f.Fill = FrameFill{Kind: FillLinear, Stops: []Stop{{Color: "red"}, {Color: "#000000"}}}
		}, "fill color"},
		{"fill stop at", func(f *FrameStyle) {
			f.Fill = FrameFill{Kind: FillLinear, Stops: []Stop{{Color: "#000000", At: 101}, {Color: "#000000"}}}
		}, "position"},
		{"fill media ok", func(f *FrameStyle) { f.Fill = FrameFill{Kind: FillMedia, Media: testMedia} }, ""},
		{"fill media missing", func(f *FrameStyle) { f.Fill.Kind = FillMedia }, "media item"},
		{"fill media malformed", func(f *FrameStyle) { f.Fill.Media = "nope" }, "media item"},
		{"pattern ok", func(f *FrameStyle) { f.Fill.Pattern = Pattern{Kind: PatternGrid, Color: "#ffffff22", Size: 400} }, ""},
		{"pattern unknown", func(f *FrameStyle) { f.Fill.Pattern.Kind = "swirl" }, "pattern"},
		{"pattern no color", func(f *FrameStyle) { f.Fill.Pattern.Kind = PatternDots }, "pattern color"},
		{"pattern size low", func(f *FrameStyle) {
			f.Fill.Pattern = Pattern{Kind: PatternDots, Color: "#ffffff", Size: 1}
		}, "pattern size"},
		{"pattern size high", func(f *FrameStyle) {
			f.Fill.Pattern = Pattern{Kind: PatternDots, Color: "#ffffff", Size: 401}
		}, "pattern size"},
		{"pattern angle", func(f *FrameStyle) { f.Fill.Pattern.Angle = -361 }, "pattern angle"},

		{"border ok", func(f *FrameStyle) { f.Border = Border{Width: 3, Style: BorderDouble, Color: "theme:border"} }, ""},
		{"border width high", func(f *FrameStyle) { f.Border = Border{Width: MaxFrameLength + 1, Color: "#000000"} }, "border width"},
		{"border width negative", func(f *FrameStyle) { f.Border.Width = -1 }, "border width"},
		{"border style", func(f *FrameStyle) { f.Border.Style = "wavy" }, "border style"},
		{"border no color", func(f *FrameStyle) { f.Border.Width = 2 }, "border color"},
		{"border gradient", func(f *FrameStyle) { f.Border = Border{Width: 2, Stops: gradient(2)} }, ""},
		{"border one stop", func(f *FrameStyle) { f.Border = Border{Width: 2, Color: "#000000", Stops: gradient(1)} }, "color stops"},
		{"border too many stops", func(f *FrameStyle) { f.Border = Border{Width: 2, Stops: gradient(MaxStops + 1)} }, "at most"},
		{"border angle", func(f *FrameStyle) { f.Border.Angle = 400 }, "border angle"},
		{"border inset out", func(f *FrameStyle) { f.Border.Inset = -MaxFrameLength }, ""},
		{"border inset high", func(f *FrameStyle) { f.Border.Inset = MaxFrameLength + 1 }, "border inset"},

		{"rings max", func(f *FrameStyle) {
			f.Rings = []Ring{{Width: 2, Gap: 4, Color: "#fff000"}, {Width: 1, Color: "theme:accent"}}
		}, ""},
		{"rings too many", func(f *FrameStyle) { f.Rings = make([]Ring, MaxRings+1) }, "at most"},
		{"ring width", func(f *FrameStyle) { f.Rings = []Ring{{Width: -1, Color: "#000000"}} }, "ring width"},
		{"ring gap", func(f *FrameStyle) { f.Rings = []Ring{{Gap: MaxFrameLength + 1, Color: "#000000"}} }, "ring gap"},
		{"ring color", func(f *FrameStyle) { f.Rings = []Ring{{Width: 1}} }, "ring color"},

		{"brackets ok", func(f *FrameStyle) { f.Brackets = Brackets{Length: 20, Width: 3, Color: "#00ffff", Inset: -4} }, ""},
		{"brackets no color", func(f *FrameStyle) { f.Brackets.Length = 10 }, "bracket color"},
		{"brackets length", func(f *FrameStyle) { f.Brackets.Length = MaxFrameLength + 1 }, "bracket length"},
		{"brackets width", func(f *FrameStyle) { f.Brackets.Width = -2 }, "bracket width"},
		{"brackets inset", func(f *FrameStyle) { f.Brackets.Inset = MaxFrameLength + 1 }, "bracket inset"},

		{"shadows max", func(f *FrameStyle) { f.Shadows = make([]BoxShadow, MaxBoxShadows); fillShadows(f.Shadows) }, ""},
		{"shadows too many", func(f *FrameStyle) { f.Shadows = make([]BoxShadow, MaxBoxShadows+1) }, "at most"},
		{"shadow x", func(f *FrameStyle) { f.Shadows = []BoxShadow{{X: MaxFrameLength + 1, Color: "#000000"}} }, "shadow x"},
		{"shadow y", func(f *FrameStyle) { f.Shadows = []BoxShadow{{Y: -MaxFrameLength - 1, Color: "#000000"}} }, "shadow y"},
		{"shadow blur", func(f *FrameStyle) { f.Shadows = []BoxShadow{{Blur: -1, Color: "#000000"}} }, "shadow blur"},
		{"shadow spread", func(f *FrameStyle) { f.Shadows = []BoxShadow{{Spread: MaxFrameLength + 1, Color: "#000000"}} }, "shadow spread"},
		{"shadow color", func(f *FrameStyle) { f.Shadows = []BoxShadow{{Inset: true}} }, "shadow color"},
		{"shadow inset ok", func(f *FrameStyle) { f.Shadows = []BoxShadow{{Spread: -3, Color: "theme:text", Inset: true}} }, ""},

		{"glow ok", func(f *FrameStyle) { f.Glow = Glow{Size: 20, Strength: 2, Color: "theme:primary"} }, ""},
		{"glow size", func(f *FrameStyle) { f.Glow = Glow{Size: 201, Strength: 1, Color: "#000000"} }, "glow size"},
		{"glow strength", func(f *FrameStyle) { f.Glow = Glow{Size: 5, Color: "#000000"} }, "glow strength"},
		{"glow color", func(f *FrameStyle) { f.Glow = Glow{Size: 5, Strength: 1, Color: "glowy"} }, "glow color"},

		{"bevel ok", func(f *FrameStyle) { f.Bevel = Bevel{Size: 2, Light: "#ffffff80", Dark: "#00000080"} }, ""},
		{"bevel size", func(f *FrameStyle) { f.Bevel.Size = MaxFrameLength + 1 }, "bevel size"},
		{"bevel light", func(f *FrameStyle) { f.Bevel = Bevel{Size: 2, Dark: "#000000"} }, "bevel light"},
		{"bevel dark", func(f *FrameStyle) { f.Bevel = Bevel{Size: 2, Light: "#000000"} }, "bevel dark"},

		{"texture ok", func(f *FrameStyle) { f.Texture = Texture{Kind: TextureGrain, Opacity: 100} }, ""},
		{"texture kind", func(f *FrameStyle) { f.Texture.Kind = "sand" }, "texture"},
		{"texture opacity", func(f *FrameStyle) { f.Texture.Opacity = 101 }, "texture opacity"},
		{"texture opacity low", func(f *FrameStyle) { f.Texture.Opacity = -1 }, "texture opacity"},

		{"image ok", func(f *FrameStyle) {
			f.Image = NineSlice{Media: testMedia, Slice: [4]int{10, 10, 10, 10}, Width: [4]int{8, 8, 8, 8}, Repeat: RepeatRound, Fill: true}
		}, ""},
		{"image slice without media ok", func(f *FrameStyle) { f.Image.Slice = [4]int{4, 4, 4, 4} }, ""},
		{"image media", func(f *FrameStyle) { f.Image.Media = "media-" }, "media item"},
		{"image slice", func(f *FrameStyle) { f.Image.Slice[1] = MaxFrameLength + 1 }, "image slice"},
		{"image slice negative", func(f *FrameStyle) { f.Image.Slice[0] = -1 }, "image slice"},
		{"image width", func(f *FrameStyle) { f.Image.Width[3] = -1 }, "image border width"},
		{"image repeat", func(f *FrameStyle) { f.Image.Repeat = "tile" }, "image repeat"},

		{"animation ok", func(f *FrameStyle) {
			f.Animation = FrameAnimation{Kind: FrameAnimShine, Speed: 0.2, Color: "theme:highlight"}
		}, ""},
		{"animation max speed", func(f *FrameStyle) { f.Animation = FrameAnimation{Kind: FrameAnimAnts, Speed: 20} }, ""},
		{"animation kind", func(f *FrameStyle) { f.Animation.Kind = "spin" }, "frame animation"},
		{"animation slow", func(f *FrameStyle) { f.Animation.Speed = 20.5 }, "speed"},
		{"animation fast", func(f *FrameStyle) { f.Animation.Speed = 0.1 }, "speed"},
		{"animation color", func(f *FrameStyle) { f.Animation.Color = "white" }, "frame animation color"},

		{"padding ok", func(f *FrameStyle) { f.Padding = MaxFrameLength }, ""},
		{"padding negative", func(f *FrameStyle) { f.Padding = -1 }, "padding"},
		{"padding high", func(f *FrameStyle) { f.Padding = MaxFrameLength + 1 }, "padding"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var f FrameStyle
			tt.mut(&f)
			f.Canon()
			err := f.Validate()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Validate = %v", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFrameValidateNaN(t *testing.T) {
	t.Parallel()
	f := FrameStyle{Animation: FrameAnimation{Speed: math.NaN()}}
	f.Canon()
	if err := f.Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("NaN speed = %v", err)
	}
}

func TestFrameTooLarge(t *testing.T) {
	t.Parallel()
	// Within every count limit, but longer than MaxFrameBytes in JSON.
	f := FrameStyle{Fill: FrameFill{Kind: FillMedia, Media: testMedia + strings.Repeat("0", MaxFrameBytes)}}
	f.Canon()
	if err := f.Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("Validate = %v", err)
	}
	if _, err := DecodeFrame(json.RawMessage(strings.Repeat(" ", MaxFrameBytes+1))); !errors.Is(err, ErrInvalid) {
		t.Errorf("DecodeFrame = %v", err)
	}
}

func TestDecodeFrame(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "null", " null ", "{}"} {
		f, err := DecodeFrame(json.RawMessage(in))
		if err != nil {
			t.Fatalf("DecodeFrame(%q) = %v", in, err)
		}
		if f.Rings == nil || f.Shadows == nil || f.Fill.Stops == nil || f.Border.Stops == nil {
			t.Errorf("DecodeFrame(%q) left nil lists: %+v", in, f)
		}
	}
	for _, in := range []string{`{"bogus":1}`, `{"padding":"x"}`, `{"padding":1} {}`, `[`, `{"padding":-3}`} {
		if _, err := DecodeFrame(json.RawMessage(in)); !errors.Is(err, ErrInvalid) {
			t.Errorf("DecodeFrame(%q) = %v", in, err)
		}
	}
	b, _ := json.Marshal(FrameStyle{})
	if !strings.Contains(string(b), `"rings":null`) {
		t.Fatalf("test assumption: the raw zero value encodes null lists: %s", b)
	}
	f, _ := DecodeFrame(nil)
	b, _ = json.Marshal(f)
	if strings.Contains(string(b), "null") {
		t.Errorf("canonical zero has nulls: %s", b)
	}
}

func TestFrameClone(t *testing.T) {
	t.Parallel()
	f := FrameStyle{
		Rings:   []Ring{{Width: 1, Color: "#000000"}},
		Shadows: []BoxShadow{{Color: "#000000"}},
		Fill:    FrameFill{Stops: []Stop{{Color: "#000000"}}},
		Border:  Border{Stops: []Stop{{Color: "#000000"}}},
	}
	c := f.Clone()
	c.Rings[0].Width, c.Shadows[0].X, c.Fill.Stops[0].At, c.Border.Stops[0].At = 9, 9, 9, 9
	if !reflect.DeepEqual(f.Rings[0], Ring{Width: 1, Color: "#000000"}) || f.Shadows[0].X != 0 || f.Fill.Stops[0].At != 0 || f.Border.Stops[0].At != 0 {
		t.Errorf("Clone shares memory: %+v", f)
	}
}
