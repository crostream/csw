package csw

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// valid returns a small package that passes Validate.
func valid() Package {
	return Package{
		CSW: Format, ID: "test.hello-world", Version: "1.2.0", API: APIBase, Name: "Hello", Description: "Says hello.",
		Author: Author{Name: "Tester", URL: "https://example.com"}, License: "MIT", Category: "basics",
		Tags: []string{"a"}, Icon: "sparkle", Size: Size{W: 300, H: 100},
		Fields:   []Prop{{Key: "text", Label: "Text", Kind: PropText}},
		Defaults: json.RawMessage(`{"text":"hi"}`), Data: []string{"chat"},
		Sample: map[string]json.RawMessage{"chat": json.RawMessage(`[]`)},
		Code:   Code{HTML: "<div>{{text}}</div>", CSS: "div{color:red}", JS: "sl.on('x',()=>{})"},
	}
}

func mustEncode(t *testing.T, p *Package) []byte {
	t.Helper()
	b, err := Encode(p)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return b
}

func TestEncodeDeterministic(t *testing.T) {
	t.Parallel()
	a, b := valid(), valid()
	a.Defaults = json.RawMessage(`{ "b": 1.50, "a": "<&>" }`)
	b.Defaults = json.RawMessage(`{"a":"<&>","b":1.50}`)
	ea, eb := mustEncode(t, &a), mustEncode(t, &b)
	if !bytes.Equal(ea, eb) {
		t.Fatalf("equal packages encode differently:\n%s\n%s", ea, eb)
	}
	if !bytes.HasSuffix(ea, []byte("}\n")) || !bytes.Contains(ea, []byte("\n  \"csw\": 1,")) {
		t.Errorf("not 2-space indented with a trailing newline:\n%s", ea)
	}
	if !bytes.Contains(ea, []byte("\"<&>\"")) {
		t.Error("HTML was escaped")
	}
	if !bytes.Contains(ea, []byte("1.50")) {
		t.Error("number written differently")
	}
	if keys := []string{`"csw"`, `"id"`, `"version"`, `"api"`, `"name"`, `"code"`}; !inOrder(string(ea), keys) {
		t.Errorf("key order wrong:\n%s", ea)
	}
}

func inOrder(s string, parts []string) bool {
	pos := 0
	for _, p := range parts {
		i := strings.Index(s[pos:], p)
		if i < 0 {
			return false
		}
		pos += i
	}
	return true
}

func TestEncodeNilsAndEmpty(t *testing.T) {
	t.Parallel()
	p := valid()
	p.Tags, p.Fields, p.Data, p.Sample, p.Defaults, p.Fields = nil, nil, nil, nil, nil, nil
	out := string(mustEncode(t, &p))
	for _, want := range []string{`"tags": []`, `"fields": []`, `"data": []`, `"sample": {}`, `"defaults": {}`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
	if strings.Contains(out, `"assets"`) || strings.Contains(out, `"alert"`) {
		t.Error("empty assets or alert were written")
	}
}

func TestEncodeErrors(t *testing.T) {
	t.Parallel()
	bad := valid()
	bad.Defaults = json.RawMessage(`{nope`)
	if _, err := Encode(&bad); err == nil {
		t.Error("bad defaults encoded")
	}
	bad = valid()
	bad.Sample = map[string]json.RawMessage{"x": json.RawMessage(`{`)}
	if _, err := Encode(&bad); err == nil {
		t.Error("bad sample encoded")
	}
	big := valid()
	big.Code.JS = strings.Repeat("a", MaxFileBytes)
	if _, err := Encode(&big); !errors.Is(err, ErrInvalid) {
		t.Errorf("oversize: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	p := valid()
	p.Alert = &WidgetAlert{Events: []string{"twitch.follow"}, Samples: map[string]map[string]string{"twitch.follow": {"user": "x"}}}
	p.API, p.Assets = APIAssets, map[string]Asset{"img/a.png": {Type: "image/png", Data: []byte("\x89PNG\r\n\x1a\nabc")}}
	enc := mustEncode(t, &p)
	got, err := Decode(bytes.NewReader(enc))
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mustEncode(t, got), enc) {
		t.Error("decode then encode changed the bytes")
	}
	if got.Assets["img/a.png"].Type != "image/png" || !bytes.Equal(got.Assets["img/a.png"].Data, []byte("\x89PNG\r\n\x1a\nabc")) {
		t.Errorf("assets: %+v", got.Assets)
	}
}

type endless struct{}

func (endless) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = ' '
	}
	return len(b), nil
}

type failing struct{}

func (failing) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestDecodeErrors(t *testing.T) {
	t.Parallel()
	base := string(mustEncode(t, ptr(valid())))
	cases := map[string]string{
		"unknown field":  strings.Replace(base, `"csw": 1,`, `"csw": 1, "extra": true,`, 1),
		"unknown nested": strings.Replace(base, `"w": 300`, `"w": 300, "z": 1`, 1),
		"trailing":       base + `{}`,
		"not json":       `<html>`,
		"wrong format":   strings.Replace(base, `"csw": 1`, `"csw": 2`, 1),
		"missing csw":    `{"id":"a.b"}`,
		"wrong type":     strings.Replace(base, `"api": 1`, `"api": "one"`, 1),
	}
	for name, in := range cases {
		if _, err := Decode(strings.NewReader(in)); !errors.Is(err, ErrFormat) {
			t.Errorf("%s: err = %v, want ErrFormat", name, err)
		}
	}
	if _, err := Decode(io.LimitReader(endless{}, MaxFileBytes+10)); !errors.Is(err, ErrFormat) {
		t.Errorf("oversize: %v", err)
	}
	if _, err := Decode(failing{}); err == nil || errors.Is(err, ErrFormat) {
		t.Errorf("read error: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func TestValidate(t *testing.T) {
	t.Parallel()
	png := func(n int) Asset {
		d := make([]byte, n)
		copy(d, "\x89PNG\r\n\x1a\n")
		return Asset{Type: "image/png", Data: d}
	}
	cases := []struct {
		name string
		mut  func(*Package)
		want string // "" = valid
	}{
		{"ok", func(*Package) {}, ""},
		{"bad format", func(p *Package) { p.CSW = 2 }, "format version"},
		{"id upper", func(p *Package) { p.ID = "Test.Hello" }, "id"},
		{"id no dot", func(p *Package) { p.ID = "hello" }, "id"},
		{"id double hyphen", func(p *Package) { p.ID = "a.b--c" }, "id"},
		{"id long", func(p *Package) { p.ID = "a." + strings.Repeat("b", MaxIDLength) }, "id"},
		{"id max", func(p *Package) { p.ID = "a." + strings.Repeat("b", MaxIDLength-2) }, ""},
		{"version", func(p *Package) { p.Version = "1.0" }, "version"},
		{"version leading zero", func(p *Package) { p.Version = "01.0.0" }, "version"},
		{"version pre", func(p *Package) { p.Version = "1.0.0-rc1" }, "version"},
		{"api 0", func(p *Package) { p.API = 0 }, "api"},
		{"api 4", func(p *Package) { p.API = 4 }, "api"},
		{"author empty", func(p *Package) { p.Author.Name = " " }, "author name"},
		{"author url", func(p *Package) { p.Author.URL = "javascript:alert(1)" }, "author url"},
		{"author url relative", func(p *Package) { p.Author.URL = "example.com" }, "author url"},
		{"author no url", func(p *Package) { p.Author.URL = "" }, ""},
		{"license empty", func(p *Package) { p.License = "" }, "license"},
		{"category", func(p *Package) { p.Category = "Bad Cat" }, "category"},
		{"category empty", func(p *Package) { p.Category = "" }, "category"},
		{"app rule: name", func(p *Package) { p.Name = "" }, "name"},
		{"app rule: size", func(p *Package) { p.Size.W = 0 }, "size"},
		{"app rule: data", func(p *Package) { p.Data = []string{"Bad Key"} }, "data"},
		{"app rule: defaults", func(p *Package) { p.Defaults = json.RawMessage(`{"text":5}`) }, "text"},
		{"app rule: code", func(p *Package) { p.Code.CSS = strings.Repeat("a", MaxWidgetCode+1) }, "CSS"},
		{"app rule: alert", func(p *Package) { p.Alert = &WidgetAlert{} }, "alert"},
		{"asset ok", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"a/b-c.d.png": png(10)} }, ""},
		{"asset needs api 3", func(p *Package) { p.Assets = map[string]Asset{"a.png": png(10)} }, "needs api 3"},
		{"asset upper", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"A.png": png(10)} }, "asset path"},
		{"asset dotdot", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"a/../b.png": png(10)} }, "asset path"},
		{"asset abs", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"/a.png": png(10)} }, "asset path"},
		{"asset hidden", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"a/.b.png": png(10)} }, "asset path"},
		{"asset dotdot name", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"a..b.png": png(10)} }, "asset path"},
		{"asset long path", func(p *Package) {
			p.API = 3
			p.Assets = map[string]Asset{strings.Repeat("a", MaxAssetPath-3) + ".png": png(10)}
		}, "asset path"},
		{"asset type", func(p *Package) {
			p.API = 3
			p.Assets = map[string]Asset{"a.exe": {Type: "application/x-msdownload", Data: []byte{1}}}
		}, "allowed"},
		{"asset ext mismatch", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"a.jpg": png(10)} }, "does not end in"},
		{"asset jpeg", func(p *Package) {
			p.API = 3
			p.Assets = map[string]Asset{"a.jpeg": {Type: "image/jpeg", Data: []byte("\xff\xd8\xff")}}
		}, ""},
		{"asset empty", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"a.png": png(0)} }, "empty"},
		{"asset max size", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"a.png": png(MaxAssetBytes)} }, ""},
		{"asset too big", func(p *Package) { p.API = 3; p.Assets = map[string]Asset{"a.png": png(MaxAssetBytes + 1)} }, "larger than"},
		{"assets total max", func(p *Package) {
			p.API = 3
			p.Assets = map[string]Asset{"a.png": png(MaxAssetBytes), "b.png": png(MaxAssetBytes), "c.png": png(MaxAssetBytes), "d.png": png(MaxAssetBytes)}
		}, ""},
		{"assets total over", func(p *Package) {
			p.API = 3
			p.Assets = map[string]Asset{"a.png": png(MaxAssetBytes), "b.png": png(MaxAssetBytes), "c.png": png(MaxAssetBytes), "d.png": png(MaxAssetBytes), "e.png": png(8)}
		}, "total"},
		{"assets count over", func(p *Package) {
			p.API = 3
			p.Assets = map[string]Asset{}
			for i := range MaxAssets + 1 {
				p.Assets[string(rune('a'+i/26))+string(rune('a'+i%26))+".png"] = png(1)
			}
		}, "at most 64"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := valid()
			c.mut(&p)
			before, _ := json.Marshal(p)
			err := p.Validate()
			after, _ := json.Marshal(p)
			if !bytes.Equal(before, after) {
				t.Error("Validate changed the package")
			}
			if c.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want ErrInvalid containing %q", err, c.want)
			}
		})
	}
}

func TestDetectAPIAndWarnings(t *testing.T) {
	t.Parallel()
	for js, want := range map[string]int{
		"":                              1,
		"sl.on('x',f)":                  1,
		"sl.sfx('a')":                   2,
		"sl.alert.done()":               2,
		"sl.leave()":                    2,
		"const u = sl.asset('a.png')":   3,
		"sl.sfx(); sl.asset('a.png')":   3,
		"mysl.alerts = 1; xsl.sfx = 2;": 1,
	} {
		p := valid()
		p.Code.JS = js
		if got := DetectAPI(&p); got != want {
			t.Errorf("DetectAPI(%q) = %d, want %d", js, got, want)
		}
	}
	p := valid()
	if w := p.Warnings(); len(w) != 0 {
		t.Errorf("clean package warns: %v", w)
	}
	p.Code.JS = "sl.sfx('x')"
	if w := p.Warnings(); len(w) != 1 || !strings.Contains(w[0], "needs api 2") {
		t.Errorf("low api: %v", w)
	}
	p.Code.JS, p.API = "", APIAlerts
	if w := p.Warnings(); len(w) != 1 || !strings.Contains(w[0], "needs only api 1") {
		t.Errorf("high api: %v", w)
	}
	p.API, p.Assets = APIAssets, map[string]Asset{"used.png": {Type: "image/png", Data: []byte{1}}, "unused.png": {Type: "image/png", Data: []byte{1}}}
	p.Code.JS = "sl.asset('used.png')"
	if w := p.Warnings(); len(w) != 1 || !strings.Contains(w[0], "unused.png") {
		t.Errorf("unused asset: %v", w)
	}
}

func TestDirRoundTrip(t *testing.T) {
	t.Parallel()
	p := valid()
	p.API = APIAssets
	p.Assets = map[string]Asset{"img/a.png": {Type: "image/png", Data: []byte("\x89PNG\r\n\x1a\n")}, "snd/b.ogg": {Type: "audio/ogg", Data: []byte("OggS")}}
	dir := filepath.Join(t.TempDir(), p.ID)
	if err := ToDir(&p, dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"manifest.json", "widget.html", "widget.css", "widget.js", "assets/img/a.png", "assets/snd/b.ogg"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	got, err := FromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mustEncode(t, got), mustEncode(t, &p)) {
		t.Error("dir round trip changed the package")
	}
	// A package without a script drops a stale widget.js, and rewriting is idempotent.
	p.Code.JS, p.Assets = "", nil
	if err := ToDir(&p, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "widget.js")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale widget.js: %v", err)
	}
	got, err = FromDir(dir)
	if err != nil || got.Code.JS != "" {
		t.Fatalf("FromDir = %+v, %v", got, err)
	}
	// Folder to .csw to folder keeps the bytes.
	p2, _ := FromDir(dir)
	enc := mustEncode(t, p2)
	dec, _ := Decode(bytes.NewReader(enc))
	dir2 := filepath.Join(t.TempDir(), "again")
	if err := ToDir(dec, dir2); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	b, _ := os.ReadFile(filepath.Join(dir2, "manifest.json"))
	if !bytes.Equal(a, b) {
		t.Error("manifest differs after dir -> csw -> dir")
	}
}

func TestFromDirErrors(t *testing.T) {
	t.Parallel()
	write := func(t *testing.T, files map[string]string) string {
		t.Helper()
		dir := t.TempDir()
		p := valid()
		if err := ToDir(&p, dir); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			full := filepath.Join(dir, filepath.FromSlash(name))
			if content == "\x00remove" {
				if err := os.Remove(full); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	manifestWith := func(old, repl string) string {
		p := valid()
		dir := t.TempDir()
		_ = ToDir(&p, dir)
		b, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
		return strings.Replace(string(b), old, repl, 1)
	}
	cases := map[string]map[string]string{
		"no manifest":      {"manifest.json": "\x00remove"},
		"no html":          {"widget.html": "\x00remove"},
		"no css":           {"widget.css": "\x00remove"},
		"code in manifest": {"manifest.json": manifestWith(`"csw": 1,`, `"csw": 1, "code": {},`)},
		"unknown field":    {"manifest.json": manifestWith(`"csw": 1,`, `"csw": 1, "x": 1,`)},
		"bad format":       {"manifest.json": manifestWith(`"csw": 1,`, `"csw": 3,`)},
		"not json":         {"manifest.json": "{"},
		"asset ext":        {"assets/a.exe": "x"},
		"huge manifest":    {"manifest.json": strings.Repeat(" ", MaxFileBytes+1)},
	}
	for name, files := range cases {
		if _, err := FromDir(write(t, files)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	// A directory where a file belongs is not a regular file.
	dir := write(t, nil)
	if err := os.Remove(filepath.Join(dir, "widget.css")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "widget.css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := FromDir(dir); err == nil {
		t.Error("directory as widget.css accepted")
	}
	// Too many and too large assets.
	dir = write(t, nil)
	for i := range MaxAssets + 1 {
		_ = os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, "assets", string(rune('a'+i/26))+string(rune('a'+i%26))+".png"), []byte{1}, 0o644)
	}
	if _, err := FromDir(dir); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Errorf("many assets: %v", err)
	}
	dir = write(t, map[string]string{"assets/big.png": strings.Repeat("x", MaxAssetBytes+1)})
	if _, err := FromDir(dir); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("big asset: %v", err)
	}
}

func TestToDirErrors(t *testing.T) {
	t.Parallel()
	p := valid()
	p.Assets = map[string]Asset{"../evil.png": {Type: "image/png", Data: []byte{1}}}
	dir := filepath.Join(t.TempDir(), "w")
	if err := ToDir(&p, dir); !errors.Is(err, ErrInvalid) {
		t.Errorf("traversal: %v", err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("wrote before validating asset paths")
	}
	p = valid()
	p.Defaults = json.RawMessage("{")
	if err := ToDir(&p, t.TempDir()); err == nil {
		t.Error("bad defaults written")
	}
	// The target is a file.
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	p = valid()
	if err := ToDir(&p, file); err == nil {
		t.Error("wrote into a file")
	}
	if err := ToDir(&p, filepath.Join(file, "sub")); err == nil {
		t.Error("wrote under a file")
	}
	// Asset path blocked by a file.
	d2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(d2, "assets"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	p.API, p.Assets = APIAssets, map[string]Asset{"a.png": {Type: "image/png", Data: []byte{1}}}
	if err := ToDir(&p, d2); err == nil {
		t.Error("asset written through a file")
	}
}
