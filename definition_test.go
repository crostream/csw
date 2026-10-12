package csw

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// baseDef is a small definition that passes ValidateDefinition.
func baseDef() Definition {
	return Definition{
		Name: "V", Size: Size{W: 300, H: 100},
		Fields:   []Prop{{Key: "text", Label: "Text", Kind: PropText}},
		Defaults: json.RawMessage(`{"text":"hi"}`),
		HTML:     "<div>{{text}}</div>", CSS: "div{}",
	}
}

func TestValidateDefinition(t *testing.T) {
	t.Parallel()
	num := Prop{Key: "n", Kind: PropNumber, Min: 0, Max: 10}
	tests := []struct {
		name string
		edit func(d *Definition)
		want string // "" = valid
	}{
		{"ok", func(*Definition) {}, ""},
		{"name empty", func(d *Definition) { d.Name = "  " }, "name"},
		{"name long", func(d *Definition) { d.Name = strings.Repeat("n", maxWidgetName+1) }, "name"},
		{"description", func(d *Definition) { d.Description = strings.Repeat("d", 501) }, "description"},
		{"icon", func(d *Definition) { d.Icon = strings.Repeat("i", 41) }, "icon"},
		{"too many tags", func(d *Definition) { d.Tags = strings.Split("a,b,c,d,e,f,g,h,i,j,k", ",") }, "tags"},
		{"long tag", func(d *Definition) { d.Tags = []string{strings.Repeat("t", 31)} }, "tag"},
		{"too many fields", func(d *Definition) {
			d.Fields = nil
			for i := range MaxWidgetFields + 1 {
				d.Fields = append(d.Fields, Prop{Key: fmt.Sprintf("f%d", i), Kind: PropText})
			}
			d.Defaults = nil
		}, "at most"},
		{"field key", func(d *Definition) { d.Fields[0].Key = "Bad" }, "key"},
		{"duplicate key", func(d *Definition) { d.Fields = append(d.Fields, d.Fields[0]) }, "twice"},
		{"unknown kind", func(d *Definition) { d.Fields[0].Kind = "wat" }, "unknown kind"},
		{"number below min", func(d *Definition) {
			n := num
			n.Min, n.Max = 5, 1
			d.Fields = []Prop{n}
			d.Defaults = json.RawMessage(`{"n":2}`)
		}, "below the minimum"},
		{"number above max", func(d *Definition) { d.Fields = []Prop{num}; d.Defaults = json.RawMessage(`{"n":11}`) }, "outside 0 to 10"},
		{"number bounds", func(d *Definition) { n := num; n.Max = 1e12; d.Fields = []Prop{n}; d.Defaults = nil }, "bounds"},
		{"number step", func(d *Definition) { n := num; n.Step = -1; d.Fields = []Prop{n}; d.Defaults = nil }, "step"},
		{"enum empty", func(d *Definition) { d.Fields = []Prop{{Key: "e", Kind: PropEnum}}; d.Defaults = nil }, "options"},
		{"enum dup", func(d *Definition) {
			d.Fields = []Prop{{Key: "e", Kind: PropEnum, Options: []Choice{{Value: "a"}, {Value: "a"}}}}
			d.Defaults = nil
		}, "unique"},
		{"enum label long", func(d *Definition) {
			d.Fields = []Prop{{Key: "e", Kind: PropEnum, Options: []Choice{{Value: "a", Label: strings.Repeat("l", 81)}}}}
			d.Defaults = nil
		}, "option labels"},
		{"media accept", func(d *Definition) {
			d.Fields = []Prop{{Key: "m", Kind: PropMedia, Accept: []string{"pdf"}}}
			d.Defaults = nil
		}, "image, video or audio"},
		{"items empty", func(d *Definition) {
			d.Fields = []Prop{{Key: "l", Kind: PropItems, Max: 3}}
			d.Defaults = nil
		}, "item field"},
		{"items nested", func(d *Definition) {
			d.Fields = []Prop{{Key: "l", Kind: PropItems, Max: 3, Item: []Prop{{Key: "i", Kind: PropItems, Max: 2, Item: []Prop{{Key: "x", Kind: PropText}}}}}}
			d.Defaults = nil
		}, "another list"},
		{"items max", func(d *Definition) {
			d.Fields = []Prop{{Key: "l", Kind: PropItems, Max: 5000, Item: []Prop{{Key: "x", Kind: PropText}}}}
			d.Defaults = nil
		}, "items"},
		{"label length", func(d *Definition) { d.Fields[0].Label = strings.Repeat("l", 81) }, "label"},
		{"maxlen", func(d *Definition) { d.Fields[0].MaxLen = -1 }, "max length"},
		{"defaults array", func(d *Definition) { d.Defaults = json.RawMessage(`[1]`) }, "defaults"},
		{"defaults null", func(d *Definition) { d.Defaults = json.RawMessage(`null`) }, ""},
		{"defaults empty", func(d *Definition) { d.Defaults = nil }, ""},
		{"defaults unknown key", func(d *Definition) { d.Defaults = json.RawMessage(`{"nope":1}`) }, "not a field"},
		{"defaults wrong type", func(d *Definition) { d.Defaults = json.RawMessage(`{"text":5}`) }, "default of"},
		{"defaults too long", func(d *Definition) { d.Defaults = json.RawMessage(`{"text":"` + strings.Repeat("x", 10_001) + `"}`) }, "default of"},
		{"defaults huge", func(d *Definition) {
			d.Defaults = json.RawMessage(`{"text":"` + strings.Repeat("x", MaxWidgetCode) + `"}`)
		}, "larger than"},
		{"code long", func(d *Definition) { d.CSS = strings.Repeat("a", MaxWidgetCode+1) }, "CSS"},
		{"too many data", func(d *Definition) {
			d.Data = nil
			for i := range MaxWidgetData + 1 {
				d.Data = append(d.Data, fmt.Sprintf("k%d", i))
			}
		}, "at most"},
		{"data twice", func(d *Definition) { d.Data = []string{"chat", "chat"} }, "twice"},
		{"data star", func(d *Definition) { d.Data = []string{"*"} }, "data key"},
		{"data glob mid", func(d *Definition) { d.Data = []string{"a.*.b"} }, "data key"},
		{"data glob end", func(d *Definition) { d.Data = []string{"latest.*"} }, ""},
		{"sample size", func(d *Definition) {
			d.Sample = map[string]json.RawMessage{"a": json.RawMessage(`"` + strings.Repeat("x", MaxWidgetSample) + `"`)}
		}, "sample"},
		{"sample empty key", func(d *Definition) { d.Sample = map[string]json.RawMessage{"": json.RawMessage(`1`)} }, "sample keys"},
		{"sample not json", func(d *Definition) { d.Sample = map[string]json.RawMessage{"a": json.RawMessage(`{`)} }, "valid JSON"},
		{"size", func(d *Definition) { d.Size.W = MaxSize + 1 }, "size"},
		{"size zero", func(d *Definition) { d.Size.H = 0 }, "size"},
		{"asset bad", func(d *Definition) { d.Assets = map[string]DefinitionAsset{"../evil.png": pngAsset()} }, "asset path"},
		{"asset ok", func(d *Definition) { d.Assets = map[string]DefinitionAsset{"img/a.png": pngAsset()} }, ""},
		{"alert bad", func(d *Definition) { d.Alert = &WidgetAlert{} }, "alert"},
		{"alert ok", func(d *Definition) { d.Alert = &WidgetAlert{Events: []string{"*"}} }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := baseDef()
			tt.edit(&d)
			err := ValidateDefinition(&d)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want ErrInvalid containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateDefinitionMessages(t *testing.T) {
	t.Parallel()
	d := baseDef()
	d.Name = ""
	err := ValidateDefinition(&d)
	if want := "invalid widget package: name must be 1 to 60 characters"; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
	d = baseDef()
	d.Fields[0].Label = strings.Repeat("l", 81)
	err = ValidateDefinition(&d)
	if want := `invalid widget package: field "text": label must be at most 80 characters`; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestValidateDefinitionNormalizes(t *testing.T) {
	t.Parallel()
	d := Definition{
		Name: "  Rich ", Size: Size{W: 1, H: 1}, Tags: []string{" one ", "", "two"},
		Fields: []Prop{
			{Key: "title", Kind: PropText},
			{Key: "n", Kind: PropNumber, Min: 1, Max: 9},
			{Key: "on", Kind: PropBool},
			{Key: "c", Kind: PropColor},
			{Key: "pick", Kind: PropEnum, Options: []Choice{{Value: "a"}, {Value: "b", Label: "Bee"}}},
			{Key: "m", Kind: PropMedia, Accept: []string{MediaImage}},
			{Key: "f", Kind: PropFont},
			{Key: "fx", Kind: PropEffects},
			{Key: "fr", Kind: PropFrame},
			{Key: "folder", Kind: PropFolder},
			{Key: "rows", Kind: PropItems, Max: 3, Item: []Prop{{Key: "name", Kind: PropText}, {Key: "v", Kind: PropNumber, Max: 5}}},
		},
		Defaults: json.RawMessage(`{"n": 3, "on": true, "c": "#112233", "pick": "b", "rows": [{"name":"a","v":2}]}`),
		Data:     []string{"chat", "latest.*", "timer.main_1"},
		Sample:   map[string]json.RawMessage{"chat": json.RawMessage(`[]`), "x": json.RawMessage(`null`)},
		Assets:   map[string]DefinitionAsset{},
	}
	if err := ValidateDefinition(&d); err != nil {
		t.Fatal(err)
	}
	if d.Name != "Rich" || len(d.Tags) != 2 || d.Tags[0] != "one" || d.Fields[4].Options[0].Label != "a" || d.Fields[0].Label != "title" ||
		strings.Contains(string(d.Defaults), " ") || d.Assets != nil {
		t.Errorf("normalized = %+v", d)
	}

	// Nil lists become empty ones; no defaults become an empty object.
	e := Definition{Name: "E", Size: Size{W: 1, H: 1}}
	if err := ValidateDefinition(&e); err != nil {
		t.Fatal(err)
	}
	if e.Tags == nil || e.Fields == nil || e.Data == nil || e.Sample == nil || string(e.Defaults) != "{}" {
		t.Errorf("not canonical: %+v", e)
	}
}

func TestNumberMaxAtOrBelowMinMeansNoUpperLimit(t *testing.T) {
	t.Parallel()
	for _, bounds := range [][2]float64{{5, 1}, {5, 5}, {0, 0}} {
		d := baseDef()
		d.Fields = []Prop{{Key: "n", Kind: PropNumber, Min: bounds[0], Max: bounds[1]}}
		d.Defaults = json.RawMessage(`{"n":1000000}`)
		if err := ValidateDefinition(&d); err != nil {
			t.Errorf("min %v max %v: %v", bounds[0], bounds[1], err)
		}
	}
	if (Prop{Min: 1, Max: 2}).HasMax() != true || (Prop{Min: 2, Max: 2}).HasMax() {
		t.Error("HasMax")
	}
}

func TestDefinitionStoredAssets(t *testing.T) {
	t.Parallel()
	digest := strings.Repeat("ab", 32)
	d := baseDef()
	d.Assets = map[string]DefinitionAsset{"a.png": {Type: "image/png", SHA256: digest, Size: 10}}
	if err := ValidateDefinition(&d); err != nil {
		t.Errorf("metadata-only asset: %v", err)
	}
	d.Assets["a.png"] = DefinitionAsset{Type: "image/png", SHA256: "XYZ", Size: 10}
	if err := ValidateDefinition(&d); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad digest: %v", err)
	}
}

func TestPropValues(t *testing.T) {
	t.Parallel()
	const media = "media-00000000000000000000000001"
	tests := []struct {
		name string
		p    Prop
		v    any
		want string // "" = valid
	}{
		{"text", Prop{Kind: PropText}, "x", ""},
		{"text long", Prop{Kind: PropText, MaxLen: 2}, "xyz", "longer than 2"},
		{"text default max", Prop{Kind: PropText}, strings.Repeat("x", defaultMaxLen+1), "longer than"},
		{"color", Prop{Kind: PropColor}, "#112233", ""},
		{"color alpha", Prop{Kind: PropColor}, "#11223344", ""},
		{"color theme", Prop{Kind: PropColor}, "theme:accent", ""},
		{"color bad", Prop{Kind: PropColor}, "red", "color"},
		{"color empty", Prop{Kind: PropColor}, "", "color"},
		{"color optional", Prop{Kind: PropColor, Optional: true}, "", ""},
		{"enum", Prop{Kind: PropEnum, Options: []Choice{{Value: "a"}}}, "a", ""},
		{"enum bad", Prop{Kind: PropEnum, Options: []Choice{{Value: "a"}}}, "b", "options"},
		{"media", Prop{Kind: PropMedia}, media, ""},
		{"media empty", Prop{Kind: PropMedia}, "", ""},
		{"media bad", Prop{Kind: PropMedia}, "x", "media item"},
		{"media wrong prefix", Prop{Kind: PropMedia}, "folder-00000000000000000000000001", "media item"},
		{"folder", Prop{Kind: PropFolder}, "folder-00000000000000000000000001", ""},
		{"folder bad", Prop{Kind: PropFolder}, media, "media folder"},
		{"font", Prop{Kind: PropFont}, "Arial, sans-serif", ""},
		{"font theme", Prop{Kind: PropFont}, "theme:heading", ""},
		{"font bad theme", Prop{Kind: PropFont}, "theme:nope", "font name"},
		{"font injection", Prop{Kind: PropFont}, "a;}", "font name"},
		{"number", Prop{Kind: PropNumber, Min: 0, Max: 5}, 3.0, ""},
		{"number high", Prop{Kind: PropNumber, Min: 0, Max: 5}, 6.0, "outside"},
		{"bool", Prop{Kind: PropBool}, true, ""},
		{"effects", Prop{Kind: PropEffects}, map[string]any{}, ""},
		{"effects bad", Prop{Kind: PropEffects}, map[string]any{"glow": map[string]any{"size": 999.0}}, "glow"},
		{"frame", Prop{Kind: PropFrame}, map[string]any{}, ""},
		{"frame bad", Prop{Kind: PropFrame}, map[string]any{"padding": -1.0}, "padding"},
		{"items", Prop{Kind: PropItems, Min: 1, Max: 2, Item: []Prop{{Key: "x", Kind: PropText}}}, []any{map[string]any{"x": "a"}}, ""},
		{"items count", Prop{Kind: PropItems, Min: 1, Max: 2, Item: []Prop{{Key: "x", Kind: PropText}}}, []any{}, "items is outside"},
		{"items value", Prop{Kind: PropItems, Min: 0, Max: 2, Item: []Prop{{Key: "x", Kind: PropNumber, Max: 1}}}, []any{map[string]any{"x": 5.0}}, "item 1 x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkProp(tt.p, tt.v)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), ErrInvalid.Error()) && (tt.p.Kind == PropEffects || tt.p.Kind == PropFrame) {
				t.Errorf("error %q carries the ErrInvalid prefix", err)
			}
		})
	}
}

func TestTypeFits(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		kind PropKind
		v    any
		ok   bool
	}{
		{PropNumber, 1.0, true}, {PropNumber, "1", false},
		{PropBool, true, true}, {PropBool, "true", false},
		{PropEffects, map[string]any{}, true}, {PropFrame, "x", false},
		{PropItems, []any{map[string]any{}}, true}, {PropItems, []any{"x"}, false}, {PropItems, "x", false},
		{PropText, "x", true}, {PropText, 1.0, false},
	} {
		if got := typeFits(tt.kind, tt.v); got != tt.ok {
			t.Errorf("typeFits(%s, %v) = %v", tt.kind, tt.v, got)
		}
	}
	for v, want := range map[string]any{"null": nil, "a boolean": true, "a number": 1.0, "a string": "s", "a list": []any{}, "an object": map[string]any{}} {
		if got := jsonType(want); got != v {
			t.Errorf("jsonType(%v) = %q, want %q", want, got, v)
		}
	}
}

func TestThemeKeys(t *testing.T) {
	t.Parallel()
	if !isThemeColor("theme:primary") || isThemeColor("theme:nope") || isThemeColor("primary") {
		t.Error("isThemeColor")
	}
	if !isThemeFont("theme:body") || isThemeFont("theme:primary") {
		t.Error("isThemeFont")
	}
	if !validFontName("Segoe UI, Arial") || validFontName("a{b}") || validFontName(strings.Repeat("a", 201)) || validFontName("theme:accent") {
		t.Error("validFontName")
	}
}

func TestCheckID(t *testing.T) {
	t.Parallel()
	if err := checkID("media-00000000000000000000000001", "media"); err != nil {
		t.Error(err)
	}
	for _, s := range []string{"", "media-1", "folder-00000000000000000000000001", "media-0000000000000000000000000i"} {
		if err := checkID(s, "media"); !errors.Is(err, ErrInvalid) {
			t.Errorf("checkID(%q) = %v", s, err)
		}
	}
}

func TestCloneSchemaAndAlert(t *testing.T) {
	t.Parallel()
	if cloneSchema(nil) != nil || cloneAlert(nil) != nil {
		t.Error("nil clones")
	}
	ps := []Prop{{Key: "a", Options: []Choice{{Value: "x"}}, Accept: []string{"image"}, Item: []Prop{{Key: "i"}}}}
	c := cloneSchema(ps)
	c[0].Options[0].Value, c[0].Accept[0], c[0].Item[0].Key = "y", "audio", "j"
	if ps[0].Options[0].Value != "x" || ps[0].Accept[0] != "image" || ps[0].Item[0].Key != "i" {
		t.Errorf("cloneSchema shares memory: %+v", ps)
	}
	a := &WidgetAlert{Events: []string{"a.b"}, Samples: map[string]map[string]string{"a.b": {"u": "1"}}}
	ca := cloneAlert(a)
	ca.Events[0], ca.Samples["a.b"]["u"] = "c.d", "2"
	if a.Events[0] != "a.b" || a.Samples["a.b"]["u"] != "1" {
		t.Errorf("cloneAlert shares memory: %+v", a)
	}
}
