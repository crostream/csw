package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crostream/csw"
)

// run runs the CLI and returns its exit code and output.
func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// mustRun runs the CLI and fails unless it exits 0.
func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	code, out, errs := run(t, args...)
	if code != 0 {
		t.Fatalf("%v: exit %d\nstdout: %s\nstderr: %s", args, code, out, errs)
	}
	return out
}

func widget(id, version string) csw.Package {
	return csw.Package{
		CSW: csw.Format, ID: id, Version: version, API: csw.APIBase, Name: "W " + id, Description: "d",
		Author: csw.Author{Name: "T"}, License: "MIT", Category: "basics", Icon: "sparkle",
		Size:     csw.Size{W: 100, H: 100},
		Defaults: json.RawMessage(`{}`),
		Code:     csw.Code{HTML: "<div>" + id + "</div>", CSS: "div{}"},
	}
}

// source writes widgets as source folders under a new root.
func source(t *testing.T, pkgs ...csw.Package) string {
	t.Helper()
	root := t.TempDir()
	for i := range pkgs {
		if err := csw.ToDir(&pkgs[i], filepath.Join(root, pkgs[i].ID)); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// keyFiles writes a throwaway keypair and returns the private and public
// key paths.
func keyFiles(t *testing.T) (priv, pub string) {
	t.Helper()
	privPEM, pubPEM, err := csw.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	priv, pub = filepath.Join(dir, "k"), filepath.Join(dir, "k.pub")
	if err := os.WriteFile(priv, privPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pub, pubPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	return priv, pub
}

func TestEndToEnd(t *testing.T) {
	t.Parallel()
	src := source(t, widget("test.one", "1.0.0"), widget("test.two", "1.0.0"))
	dist := filepath.Join(t.TempDir(), "dist")
	priv, pub := keyFiles(t)

	if out := mustRun(t, "check", src); strings.Count(out, "ok   ") != 2 {
		t.Errorf("check output:\n%s", out)
	}
	// Flags may follow, precede or sit between the positional arguments.
	if out := mustRun(t, "pack", src, "-o", dist); strings.Count(out, "packed") != 2 {
		t.Errorf("pack output:\n%s", out)
	}
	if out := mustRun(t, "pack", "-o", dist, src); strings.Count(out, "unchanged") != 2 {
		t.Errorf("repack output:\n%s", out)
	}
	if out := mustRun(t, "index", dist, "-key", priv, "-featured", "test.two, ,", "-now", "2026-01-02T03:04:05Z"); !strings.Contains(out, "indexed 2 widgets (2 versions)") {
		t.Errorf("index output:\n%s", out)
	}
	if out := mustRun(t, "verify", dist, "-pub", pub); !strings.Contains(out, "2 files") {
		t.Errorf("verify output:\n%s", out)
	}
	ix, err := csw.ReadIndex(dist)
	if err != nil {
		t.Fatal(err)
	}
	if ix.Generated != "2026-01-02T03:04:05Z" || ix.Widgets[0].Featured || !ix.Widgets[1].Featured {
		t.Errorf("index = %+v", ix)
	}
	if out := mustRun(t, "check", filepath.Join(dist, "w", "test.one", "1.0.0.csw")); !strings.Contains(out, "ok   test.one 1.0.0") {
		t.Errorf("check .csw:\n%s", out)
	}

	// A new version is added; the index is rebuilt keeping old published times.
	next := widget("test.one", "1.1.0")
	if err := csw.ToDir(&next, filepath.Join(src, "test.one")); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "pack", src, "-o", dist)
	mustRun(t, "index", dist, "-key", priv, "-now", "2027-01-01T00:00:00Z")
	mustRun(t, "verify", dist, "-pub", pub)
	ix, _ = csw.ReadIndex(dist)
	if v := ix.Widgets[0].Versions; len(v) != 2 || v[1].Published != "2026-01-02T03:04:05Z" || v[0].Published != "2027-01-01T00:00:00Z" {
		t.Errorf("versions = %+v", v)
	}
	// Public key given as base64 text works too.
	pubRaw, _ := os.ReadFile(pub)
	k, _ := csw.ParsePublicKey(pubRaw)
	mustRun(t, "verify", dist, "-pub", base64.StdEncoding.EncodeToString(k))

	// Changing a published version without bumping it fails the pack.
	same := widget("test.two", "1.0.0")
	same.Description = "changed"
	if err := csw.ToDir(&same, filepath.Join(src, "test.two")); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run(t, "pack", src, "-o", dist)
	if code != 1 || !strings.Contains(out, "immutable") || !strings.Contains(out, "FAIL") {
		t.Errorf("immutable: exit %d\n%s", code, out)
	}

	// Tampering with a published file fails verify.
	f := filepath.Join(dist, "w", "test.one", "1.0.0.csw")
	b, _ := os.ReadFile(f)
	if err := os.WriteFile(f, bytes.Replace(b, []byte("</div>"), []byte("</b>"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := run(t, "verify", dist, "-pub", pub); code != 1 || !strings.Contains(out, "sha256 differs") {
		t.Errorf("tampered verify: exit %d\n%s", code, out)
	}
	_, otherPub := keyFiles(t)
	if code, out, _ := run(t, "verify", dist, "-pub", otherPub); code != 1 || !strings.Contains(out, "does not verify") {
		t.Errorf("wrong key verify: exit %d\n%s", code, out)
	}
}

func TestCheckFailures(t *testing.T) {
	t.Parallel()
	good, bad := widget("test.good", "1.0.0"), widget("test.bad", "1.0.0")
	bad.Version = "x"
	src := source(t, good, bad)
	// A folder named differently from its id.
	wrong := widget("test.named", "1.0.0")
	if err := csw.ToDir(&wrong, filepath.Join(src, "other-name")); err != nil {
		t.Fatal(err)
	}
	// A widget using a newer API than it declares warns but passes.
	low := widget("test.low", "1.0.0")
	low.Code.JS = "sl.sfx('a')"
	if err := csw.ToDir(&low, filepath.Join(src, "test.low")); err != nil {
		t.Fatal(err)
	}
	code, out, errs := run(t, "check", src)
	if code != 1 {
		t.Errorf("exit %d", code)
	}
	for _, want := range []string{"ok   test.good 1.0.0", "FAIL " + filepath.Join(src, "test.bad"), "named \"other-name\"", "warn test.low: declares api 1 but its script needs api 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if !strings.Contains(errs, "some widgets failed") {
		t.Errorf("stderr = %q", errs)
	}
	// pack reports the same failures and still packs the good ones.
	dist := t.TempDir()
	code, out, _ = run(t, "pack", src, "-o", dist)
	if code != 1 || !strings.Contains(out, "packed    test.good") || !strings.Contains(out, "warn test.low") {
		t.Errorf("pack: exit %d\n%s", code, out)
	}
	// A broken .csw file.
	badFile := filepath.Join(t.TempDir(), "x.csw")
	_ = os.WriteFile(badFile, []byte("{}"), 0o644)
	if code, _, _ := run(t, "check", badFile); code != 1 {
		t.Errorf("bad csw: exit %d", code)
	}
	if code, _, _ := run(t, "check", filepath.Join(t.TempDir(), "missing")); code != 1 {
		t.Errorf("missing: exit %d", code)
	}
	empty := t.TempDir()
	if code, _, _ := run(t, "check", empty); code != 1 {
		t.Errorf("empty root: exit %d", code)
	}
	if code, _, _ := run(t, "pack", empty, "-o", t.TempDir()); code != 1 {
		t.Errorf("empty pack: exit %d", code)
	}
	if code, _, _ := run(t, "pack", filepath.Join(empty, "nope"), "-o", t.TempDir()); code != 1 {
		t.Errorf("missing pack: exit %d", code)
	}
}

func TestUsage(t *testing.T) {
	t.Parallel()
	priv, _ := keyFiles(t)
	for name, args := range map[string][]string{
		"none":            {},
		"unknown":         {"frobnicate"},
		"pack no -o":      {"pack", "src"},
		"pack no dir":     {"pack", "-o", "x"},
		"pack two dirs":   {"pack", "a", "b", "-o", "x"},
		"pack bad flag":   {"pack", "-zzz"},
		"check none":      {"check"},
		"index no key":    {"index", "dist"},
		"index no dist":   {"index", "-key", priv},
		"verify no pub":   {"verify", "dist"},
		"genkey no -out":  {"genkey"},
		"genkey extra":    {"genkey", "-out", "x", "y"},
		"genkey bad flag": {"genkey", "-zzz"},
	} {
		if code, _, _ := run(t, args...); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
	for _, args := range [][]string{{"help"}, {"-h"}, {"pack", "-h"}, {"index", "-h"}, {"genkey", "-h"}} {
		if code, _, _ := run(t, args...); code != 0 {
			t.Errorf("%v: exit %d, want 0", args, code)
		}
	}
	if _, out, _ := run(t, "help"); !strings.Contains(out, "genkey") {
		t.Errorf("help: %s", out)
	}
}

func TestIndexFailures(t *testing.T) {
	t.Parallel()
	src := source(t, widget("test.one", "1.0.0"))
	dist := t.TempDir()
	mustRun(t, "pack", src, "-o", dist)
	priv, pub := keyFiles(t)
	notKey := filepath.Join(t.TempDir(), "junk")
	_ = os.WriteFile(notKey, []byte("junk"), 0o600)
	for name, args := range map[string][]string{
		"bad -now":         {"index", dist, "-key", priv, "-now", "yesterday"},
		"missing key":      {"index", dist, "-key", filepath.Join(t.TempDir(), "none")},
		"junk key":         {"index", dist, "-key", notKey},
		"unknown featured": {"index", dist, "-key", priv, "-featured", "nope.x"},
		"no widgets dir":   {"index", t.TempDir(), "-key", priv},
	} {
		if code, _, errs := run(t, args...); code != 1 || errs == "" {
			t.Errorf("%s: exit %d, stderr %q", name, code, errs)
		}
	}
	// A corrupt previous index stops the run rather than being replaced.
	_ = os.WriteFile(filepath.Join(dist, "index.json"), []byte("junk"), 0o644)
	if code, _, _ := run(t, "index", dist, "-key", priv); code != 1 {
		t.Errorf("corrupt previous index: exit %d", code)
	}
	if code, _, _ := run(t, "verify", dist, "-pub", pub); code != 1 {
		t.Errorf("verify without a good index: exit %d", code)
	}
	if code, _, _ := run(t, "verify", dist, "-pub", "not a key"); code != 1 {
		t.Errorf("verify with junk key: exit %d", code)
	}
}

func TestGenkey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	key := filepath.Join(dir, "keys", "k")
	if out := mustRun(t, "genkey", "-out", key); !strings.Contains(out, key+".pub") {
		t.Errorf("output: %s", out)
	}
	if st, err := os.Stat(key); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("private key: %v, %v", st, err)
	}
	before, _ := os.ReadFile(key)
	if _, err := csw.ParsePrivateKey(before); err != nil {
		t.Errorf("private key does not parse: %v", err)
	}
	if code, _, errs := run(t, "genkey", "-out", key); code != 1 || !strings.Contains(errs, "exists") {
		t.Errorf("overwrite: exit %d, %q", code, errs)
	}
	if again, _ := os.ReadFile(key); !bytes.Equal(before, again) {
		t.Error("an existing key was replaced without -force")
	}
	// -force replaces it and narrows a loose mode.
	if err := os.Chmod(key, 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "genkey", "-out", key, "-force")
	if st, _ := os.Stat(key); st.Mode().Perm() != 0o600 {
		t.Errorf("mode after -force = %v", st.Mode().Perm())
	}
	if again, _ := os.ReadFile(key); bytes.Equal(before, again) {
		t.Error("-force kept the old key")
	}
	// The pair works end to end.
	src := source(t, widget("test.one", "1.0.0"))
	dist := t.TempDir()
	mustRun(t, "pack", src, "-o", dist)
	mustRun(t, "index", dist, "-key", key)
	mustRun(t, "verify", dist, "-pub", key+".pub")
	// A key path under a file cannot be created.
	file := filepath.Join(dir, "f")
	_ = os.WriteFile(file, nil, 0o644)
	if code, _, _ := run(t, "genkey", "-out", filepath.Join(file, "sub", "k")); code != 1 {
		t.Errorf("key under a file: exit %d", code)
	}
}
