package csw

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

var (
	pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 40)...)
	oggBytes = append([]byte("OggS"), bytes.Repeat([]byte{1}, 40)...)
)

func pngAsset() DefinitionAsset { return DefinitionAsset{Type: "image/png", Data: pngBytes} }

func TestCheckAssets(t *testing.T) {
	t.Parallel()
	many := map[string]DefinitionAsset{}
	for i := range MaxAssets + 1 {
		many["a"+strings.Repeat("b", i)+".png"] = pngAsset()
	}
	big := DefinitionAsset{Type: "image/png", Data: append(bytes.Clone(pngBytes), make([]byte, MaxAssetBytes)...)}
	chunk := DefinitionAsset{Type: "image/png", Data: append(bytes.Clone(pngBytes), make([]byte, MaxAssetBytes-100)...)}
	total := map[string]DefinitionAsset{}
	for i := range 5 {
		total[string(rune('a'+i))+".png"] = chunk
	}
	cases := []struct {
		name string
		m    map[string]DefinitionAsset
		ok   bool
	}{
		{"none", nil, true},
		{"png", map[string]DefinitionAsset{"img/logo.png": pngAsset()}, true},
		{"ogg", map[string]DefinitionAsset{"sfx/hit.ogg": {Type: "audio/ogg", Data: oggBytes}}, true},
		{"metadata only", map[string]DefinitionAsset{"a.png": {Type: "image/png", SHA256: strings.Repeat("ab", 32), Size: 10}}, true},
		{"upper case path", map[string]DefinitionAsset{"Img.png": pngAsset()}, false},
		{"dotdot", map[string]DefinitionAsset{"a/../b.png": pngAsset()}, false},
		{"dotdot inside", map[string]DefinitionAsset{"a..b.png": pngAsset()}, false},
		{"leading slash", map[string]DefinitionAsset{"/a.png": pngAsset()}, false},
		{"dot segment", map[string]DefinitionAsset{"a/./b.png": pngAsset()}, false},
		{"empty path", map[string]DefinitionAsset{"": pngAsset()}, false},
		{"long path", map[string]DefinitionAsset{strings.Repeat("a", MaxAssetPath) + ".png": pngAsset()}, false},
		{"bad type", map[string]DefinitionAsset{"a.html": {Type: "text/html", Data: []byte("<b>")}}, false},
		{"svg type", map[string]DefinitionAsset{"a.svg": {Type: "image/svg+xml", Data: []byte(`<svg xmlns="x"></svg>`)}}, true},
		{"svg without svg", map[string]DefinitionAsset{"a.svg": {Type: "image/svg+xml", Data: []byte(`<html>`)}}, false},
		{"lying type", map[string]DefinitionAsset{"a.png": {Type: "image/png", Data: []byte("<script>")}}, false},
		{"empty", map[string]DefinitionAsset{"a.png": {Type: "image/png", Data: []byte{}}}, false},
		{"no data", map[string]DefinitionAsset{"a.png": {Type: "image/png"}}, false},
		{"bad digest", map[string]DefinitionAsset{"a.png": {Type: "image/png", SHA256: "xyz", Size: 3}}, false},
		{"too big", map[string]DefinitionAsset{"a.png": big}, false},
		{"too many", many, false},
		{"total", total, false},
	}
	for _, c := range cases {
		err := checkAssets(c.m)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", c.name, err)
		}
	}
}

func sniffAll(t *testing.T) {
	t.Helper()
	for typ, data := range map[string]string{
		"image/jpeg": "\xff\xd8\xff\xe0", "image/gif": "GIF89a....", "image/webp": "RIFF\x00\x00\x00\x00WEBPVP8 ",
		"audio/wav": "RIFF\x00\x00\x00\x00WAVEfmt ", "audio/mpeg": "ID3\x04", "font/woff2": "wOF2....",
	} {
		if !sniffsAs(typ, []byte(data)) {
			t.Errorf("%s not recognized", typ)
		}
		if sniffsAs(typ, []byte("nope")) {
			t.Errorf("%s accepted junk", typ)
		}
	}
	if !sniffsAs("audio/mpeg", []byte{0xff, 0xfb, 0x90}) || sniffsAs("video/mp4", []byte("x")) {
		t.Error("mpeg frame sync / unknown type")
	}
}

func TestSniffs(t *testing.T) { t.Parallel(); sniffAll(t) }
