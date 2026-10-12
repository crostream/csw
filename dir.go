package csw

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Names of the files in a gallery source folder.
const (
	ManifestFile = "manifest.json"
	HTMLFile     = "widget.html"
	CSSFile      = "widget.css"
	JSFile       = "widget.js"
	AssetsDir    = "assets"

	maxSourceFile = MaxFileBytes // a manifest or code file is read up to this
)

// FromDir reads a gallery source folder: manifest.json (the package's
// metadata, strictly decoded), widget.html and widget.css (required),
// widget.js (optional) and the files under assets/, whose types come from
// their extensions. It does not validate the package: call Validate.
func FromDir(dir string) (*Package, error) {
	raw, err := readSource(dir, ManifestFile, true)
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := strictDecode(raw, &m); err != nil {
		return nil, fmt.Errorf("csw: %s: %w: %w", ManifestFile, ErrFormat, err)
	}
	if m.CSW != Format {
		return nil, fmt.Errorf("csw: %s: %w: unsupported format version %d (want %d)", ManifestFile, ErrFormat, m.CSW, Format)
	}
	p := fromManifest(&m)
	for _, f := range []struct {
		name     string
		required bool
		dst      *string
	}{{HTMLFile, true, &p.Code.HTML}, {CSSFile, true, &p.Code.CSS}, {JSFile, false, &p.Code.JS}} {
		b, err := readSource(dir, f.name, f.required)
		if err != nil {
			return nil, err
		}
		*f.dst = string(b)
	}
	if p.Assets, err = readAssets(filepath.Join(dir, AssetsDir)); err != nil {
		return nil, err
	}
	if len(p.Assets) == 0 {
		p.Assets = nil
	}
	return &p, nil
}

// readSource reads one file of a source folder; a missing optional file
// reads as empty.
func readSource(dir, name string, required bool) ([]byte, error) {
	f, err := os.Open(filepath.Join(dir, name)) //nolint:gosec // the caller names the folder
	if err != nil {
		if !required && errors.Is(err, fs.ErrNotExist) {
			return []byte{}, nil
		}
		return nil, fmt.Errorf("csw: read %s: %w", name, err)
	}
	defer func() { _ = f.Close() }()
	b, err := readLimited(f, maxSourceFile)
	if err != nil {
		return nil, fmt.Errorf("csw: read %s: %w", name, err)
	}
	return b, nil
}

// readLimited reads all of f, failing if it holds more than limit bytes.
func readLimited(f *os.File, limit int64) ([]byte, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("larger than %d bytes", limit)
	}
	return b, nil
}

// readAssets reads every file under dir as an asset keyed by its slash path.
// A missing folder is an empty map.
func readAssets(dir string) (map[string]Asset, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return map[string]Asset{}, nil
	}
	byExt := map[string]string{}
	for typ, exts := range AssetTypes() {
		for _, e := range exts {
			byExt[e] = typ
		}
	}
	out := map[string]Asset{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		typ, ok := byExt[strings.ToLower(path.Ext(name))]
		if !ok {
			return fmt.Errorf("asset %q: no known type for its extension", name)
		}
		if len(out) >= MaxAssets {
			return fmt.Errorf("more than %d assets", MaxAssets)
		}
		f, err := os.Open(p) //nolint:gosec // found by walking the caller's folder
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		b, err := readLimited(f, MaxAssetBytes)
		if err != nil {
			return fmt.Errorf("asset %q: %w", name, err)
		}
		out[name] = Asset{Type: typ, Data: b}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("csw: read assets: %w", err)
	}
	return out, nil
}

// ToDir writes p as a gallery source folder in dir, creating it. It
// overwrites the files it writes (and removes widget.js if p has no script)
// but leaves other files alone. Asset paths must be valid, so nothing is
// written outside dir.
func ToDir(p *Package, dir string) error {
	c, err := canonical(p)
	if err != nil {
		return err
	}
	for name := range c.Assets {
		if err := checkAssetPath(name); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // source folders are not secret
		return fmt.Errorf("csw: write %s: %w", dir, err)
	}
	m, err := marshal(toManifest(&c))
	if err != nil {
		return err
	}
	files := []struct {
		name string
		data []byte
	}{{ManifestFile, m}, {HTMLFile, []byte(c.Code.HTML)}, {CSSFile, []byte(c.Code.CSS)}}
	for _, f := range files {
		if err := writeFile(filepath.Join(dir, f.name), f.data); err != nil {
			return err
		}
	}
	if c.Code.JS != "" {
		err = writeFile(filepath.Join(dir, JSFile), []byte(c.Code.JS))
	} else if rmErr := os.Remove(filepath.Join(dir, JSFile)); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
		err = fmt.Errorf("csw: remove stale %s: %w", JSFile, rmErr)
	}
	if err != nil {
		return err
	}
	for _, name := range sortedKeys(c.Assets) {
		target := filepath.Join(dir, AssetsDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec // as above
			return fmt.Errorf("csw: write asset %s: %w", name, err)
		}
		if err := writeFile(target, c.Assets[name].Data); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(name string, data []byte) error {
	if err := os.WriteFile(name, data, 0o644); err != nil { //nolint:gosec // source files are not secret
		return fmt.Errorf("csw: write %s: %w", name, err)
	}
	return nil
}
