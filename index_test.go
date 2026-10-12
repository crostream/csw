package csw

import (
	"crypto/ed25519"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 12, 9, 30, 0, 123, time.FixedZone("x", 3600))

// publish writes p into dist the way pack does and returns the file's path.
func publish(t *testing.T, dist string, p *Package) string {
	t.Helper()
	name := filepath.Join(dist, filepath.FromSlash(VersionPath(p.ID, p.Version)))
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, mustEncode(t, p), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

func keys(t *testing.T) (ed25519.PrivateKey, ed25519.PublicKey) {
	t.Helper()
	privPEM, pubPEM, err := GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := ParsePrivateKey(privPEM)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ParsePublicKey(pubPEM)
	if err != nil {
		t.Fatal(err)
	}
	return priv, pub
}

func TestBuildIndexAndVerify(t *testing.T) {
	t.Parallel()
	dist := t.TempDir()
	a1, a2, b := valid(), valid(), valid()
	a1.Version, a2.Version, a2.API = "1.9.0", "1.10.0", APIAlerts
	a2.Code.JS = "sl.sfx('x')"
	a2.Alert = &WidgetAlert{Events: []string{"twitch.follow"}}
	a2.Tags = nil
	b.ID = "test.another"
	publish(t, dist, &a1)
	publish(t, dist, &a2)
	publish(t, dist, &b)

	ix, err := BuildIndex(dist, IndexOptions{Now: t0, Featured: []string{"test.another"}})
	if err != nil {
		t.Fatal(err)
	}
	if ix.V != 1 || ix.Generated != "2026-10-12T08:30:00Z" || len(ix.Widgets) != 2 {
		t.Fatalf("index = %+v", ix)
	}
	another, hello := ix.Widgets[0], ix.Widgets[1]
	if another.ID != "test.another" || hello.ID != "test.hello-world" {
		t.Fatalf("not sorted by id: %s, %s", another.ID, hello.ID)
	}
	if !another.Featured || hello.Featured {
		t.Error("featured flags wrong")
	}
	if hello.Latest != "1.10.0" || hello.API != APIAlerts || len(hello.Versions) != 2 || hello.Versions[0].Version != "1.10.0" {
		t.Errorf("latest/ordering wrong: %+v", hello)
	}
	if hello.Alert == nil || hello.Alert.Events[0] != "twitch.follow" || another.Alert != nil {
		t.Errorf("alert: %+v / %+v", hello.Alert, another.Alert)
	}
	if hello.Tags == nil || hello.Author != "Tester" {
		t.Errorf("tags/author: %+v", hello)
	}
	v := hello.Versions[1]
	if v.URL != "w/test.hello-world/1.9.0.csw" || len(v.SHA256) != 64 || v.Bytes == 0 || v.Published != "2026-10-12T08:30:00Z" || v.API != 1 {
		t.Errorf("version = %+v", v)
	}

	priv, pub := keys(t)
	data, err := WriteIndex(dist, ix, priv)
	if err != nil {
		t.Fatal(err)
	}
	if n, problems := VerifyDist(dist, pub); n != 3 || len(problems) != 0 {
		t.Fatalf("VerifyDist = %d, %v", n, problems)
	}
	sig, _ := os.ReadFile(filepath.Join(dist, IndexSigFile))
	if err := VerifyIndex(data, string(sig), pub); err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), data...)
	tampered[10] ^= 1
	if err := VerifyIndex(tampered, string(sig), pub); !errors.Is(err, ErrSignature) {
		t.Errorf("tampered: %v", err)
	}
	if err := VerifyIndex(data, "***", pub); !errors.Is(err, ErrSignature) {
		t.Errorf("bad base64: %v", err)
	}
	_, other := keys(t)
	if err := VerifyIndex(data, string(sig), other); !errors.Is(err, ErrSignature) {
		t.Errorf("wrong key: %v", err)
	}

	// Rebuilding with the previous index keeps published times and bytes.
	prev, err := ReadIndex(dist)
	if err != nil {
		t.Fatal(err)
	}
	ix2, err := BuildIndex(dist, IndexOptions{Now: t0.Add(48 * time.Hour), Previous: prev, Featured: []string{"test.another"}})
	if err != nil {
		t.Fatal(err)
	}
	again, _ := ix2.Marshal()
	if string(again) == string(data) {
		t.Error("generated time should differ")
	}
	ix2.Generated = ix.Generated
	if got, _ := ix2.Marshal(); string(got) != string(data) {
		t.Error("published times changed on rebuild")
	}
	// A new version gets the new time.
	a3 := valid()
	a3.Version = "2.0.0"
	publish(t, dist, &a3)
	ix3, err := BuildIndex(dist, IndexOptions{Now: t0.Add(48 * time.Hour), Previous: prev})
	if err != nil {
		t.Fatal(err)
	}
	if ix3.Widgets[1].Versions[0].Published != "2026-10-14T08:30:00Z" || ix3.Widgets[1].Versions[1].Published != "2026-10-12T08:30:00Z" {
		t.Errorf("published = %+v", ix3.Widgets[1].Versions)
	}
}

func TestBuildIndexErrors(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (string, *Package) {
		t.Helper()
		dist := t.TempDir()
		p := valid()
		publish(t, dist, &p)
		return dist, &p
	}
	t.Run("immutable", func(t *testing.T) {
		t.Parallel()
		dist, p := setup(t)
		ix, err := BuildIndex(dist, IndexOptions{Now: t0})
		if err != nil {
			t.Fatal(err)
		}
		p.Description = "changed"
		publish(t, dist, p)
		if _, err := BuildIndex(dist, IndexOptions{Now: t0, Previous: ix}); err == nil || !strings.Contains(err.Error(), "immutable") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("unknown featured", func(t *testing.T) {
		t.Parallel()
		dist, _ := setup(t)
		if _, err := BuildIndex(dist, IndexOptions{Now: t0, Featured: []string{"nope.x"}}); err == nil {
			t.Error("no error")
		}
	})
	t.Run("no w dir", func(t *testing.T) {
		t.Parallel()
		if _, err := BuildIndex(t.TempDir(), IndexOptions{Now: t0}); err == nil {
			t.Error("no error")
		}
	})
	t.Run("stray file in w", func(t *testing.T) {
		t.Parallel()
		dist, _ := setup(t)
		_ = os.WriteFile(filepath.Join(dist, "w", "README"), nil, 0o644)
		if _, err := BuildIndex(dist, IndexOptions{Now: t0}); err == nil {
			t.Error("no error")
		}
	})
	t.Run("stray file in id dir", func(t *testing.T) {
		t.Parallel()
		dist, p := setup(t)
		_ = os.WriteFile(filepath.Join(dist, "w", p.ID, "notes.txt"), nil, 0o644)
		if _, err := BuildIndex(dist, IndexOptions{Now: t0}); err == nil {
			t.Error("no error")
		}
	})
	t.Run("empty id dir", func(t *testing.T) {
		t.Parallel()
		dist, _ := setup(t)
		_ = os.Mkdir(filepath.Join(dist, "w", "test.empty"), 0o755)
		if _, err := BuildIndex(dist, IndexOptions{Now: t0}); err == nil {
			t.Error("no error")
		}
	})
	t.Run("wrong path", func(t *testing.T) {
		t.Parallel()
		dist, p := setup(t)
		old := filepath.Join(dist, "w", p.ID, p.Version+".csw")
		if err := os.Rename(old, filepath.Join(dist, "w", p.ID, "9.9.9.csw")); err != nil {
			t.Fatal(err)
		}
		if _, err := BuildIndex(dist, IndexOptions{Now: t0}); err == nil {
			t.Error("no error")
		}
	})
	t.Run("invalid package", func(t *testing.T) {
		t.Parallel()
		dist, p := setup(t)
		p.License = ""
		publish(t, dist, p)
		if _, err := BuildIndex(dist, IndexOptions{Now: t0}); !errors.Is(err, ErrInvalid) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("not a csw", func(t *testing.T) {
		t.Parallel()
		dist, p := setup(t)
		_ = os.WriteFile(filepath.Join(dist, "w", p.ID, p.Version+".csw"), []byte("hi"), 0o644)
		if _, err := BuildIndex(dist, IndexOptions{Now: t0}); !errors.Is(err, ErrFormat) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestVerifyDistProblems(t *testing.T) {
	t.Parallel()
	priv, pub := keys(t)
	build := func(t *testing.T) (string, *Index) {
		t.Helper()
		dist := t.TempDir()
		p := valid()
		publish(t, dist, &p)
		ix, err := BuildIndex(dist, IndexOptions{Now: t0})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := WriteIndex(dist, ix, priv); err != nil {
			t.Fatal(err)
		}
		return dist, ix
	}
	file := func(dist string) string { return filepath.Join(dist, "w", "test.hello-world", "1.2.0.csw") }
	expect := func(t *testing.T, dist, want string) {
		t.Helper()
		_, problems := VerifyDist(dist, pub)
		if len(problems) == 0 {
			t.Fatalf("no problems, want %q", want)
		}
		if !strings.Contains(problems[0].Error(), want) {
			t.Errorf("problem = %v, want %q", problems[0], want)
		}
	}
	t.Run("tampered file", func(t *testing.T) {
		t.Parallel()
		dist, _ := build(t)
		b, _ := os.ReadFile(file(dist))
		_ = os.WriteFile(file(dist), []byte(strings.Replace(string(b), "Says hello.", "Says h3llo.", 1)), 0o644)
		expect(t, dist, "sha256 differs")
	})
	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		dist, _ := build(t)
		_ = os.Remove(file(dist))
		expect(t, dist, "no such file")
	})
	t.Run("unlisted file", func(t *testing.T) {
		t.Parallel()
		dist, _ := build(t)
		q := valid()
		q.Version = "1.3.0"
		publish(t, dist, &q)
		expect(t, dist, "is not in the index")
	})
	t.Run("unlisted folder", func(t *testing.T) {
		t.Parallel()
		dist, _ := build(t)
		_ = os.Mkdir(filepath.Join(dist, "w", "test.extra"), 0o755)
		expect(t, dist, "w/test.extra is not in the index")
	})
	t.Run("tampered index", func(t *testing.T) {
		t.Parallel()
		dist, _ := build(t)
		b, _ := os.ReadFile(filepath.Join(dist, IndexFile))
		_ = os.WriteFile(filepath.Join(dist, IndexFile), []byte(strings.Replace(string(b), `"featured": false`, `"featured": true`, 1)), 0o644)
		expect(t, dist, "does not verify")
	})
	t.Run("missing sig", func(t *testing.T) {
		t.Parallel()
		dist, _ := build(t)
		_ = os.Remove(filepath.Join(dist, IndexSigFile))
		expect(t, dist, "no such file")
	})
	t.Run("missing index", func(t *testing.T) {
		t.Parallel()
		expect(t, t.TempDir(), "no such file")
	})
	// Re-signed indexes with inconsistent content.
	resign := func(t *testing.T, dist string, ix *Index) {
		t.Helper()
		if _, err := WriteIndex(dist, ix, priv); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("bytes", func(t *testing.T) {
		t.Parallel()
		dist, ix := build(t)
		ix.Widgets[0].Versions[0].Bytes++
		resign(t, dist, ix)
		expect(t, dist, "bytes")
	})
	t.Run("api", func(t *testing.T) {
		t.Parallel()
		dist, ix := build(t)
		ix.Widgets[0].Versions[0].API = 3
		resign(t, dist, ix)
		expect(t, dist, "api 1")
	})
	t.Run("url", func(t *testing.T) {
		t.Parallel()
		dist, ix := build(t)
		ix.Widgets[0].Versions[0].URL = "w/other.csw"
		resign(t, dist, ix)
		expect(t, dist, "url")
	})
	t.Run("latest", func(t *testing.T) {
		t.Parallel()
		dist, ix := build(t)
		ix.Widgets[0].Latest = "9.0.0"
		resign(t, dist, ix)
		expect(t, dist, "latest")
	})
	t.Run("unsupported index version", func(t *testing.T) {
		t.Parallel()
		dist, ix := build(t)
		ix.V = 2
		resign(t, dist, ix)
		expect(t, dist, "unsupported")
	})
	t.Run("unknown index field", func(t *testing.T) {
		t.Parallel()
		dist, ix := build(t)
		data, _ := ix.Marshal()
		data = []byte(strings.Replace(string(data), `"v": 1,`, `"v": 1, "x": 0,`, 1))
		_ = os.WriteFile(filepath.Join(dist, IndexFile), data, 0o644)
		_ = os.WriteFile(filepath.Join(dist, IndexSigFile), []byte(SignIndex(data, priv)), 0o644)
		expect(t, dist, "not a valid")
	})
	if _, err := ReadIndex(t.TempDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadIndex missing: %v", err)
	}
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a, b string
		want int
	}{{"1.0.0", "1.0.0", 0}, {"1.2.0", "1.10.0", -1}, {"2.0.0", "1.99.99", 1}, {"1.0.1", "1.0.0", 1}, {"1.0", "1.0.0", -1}} {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
