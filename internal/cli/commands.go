package cli

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/crostream/csw"
)

// pack validates source folders and writes their .csw files.
func pack(args []string, stdout, stderr io.Writer) error {
	flags := newFlags("pack", "<src-root> -o <dist>", stderr)
	out := flags.String("o", "", "the dist `directory`; files go to <dist>/w/<id>/<version>.csw")
	pos, err := parse(flags, args)
	if err != nil {
		return err
	}
	if err := required(flags, "o"); err != nil {
		return err
	}
	dirs, err := sourceFolders(pos[0])
	if err != nil {
		return err
	}
	failed := 0
	for _, dir := range dirs {
		msg, err := packOne(dir, *out)
		if err != nil {
			failed++
			_, _ = fmt.Fprintf(stdout, "FAIL %s: %v\n", dir, err)
			continue
		}
		_, _ = fmt.Fprint(stdout, msg)
	}
	if failed > 0 {
		return fmt.Errorf("%w: %d of %d", errFailed, failed, len(dirs))
	}
	return nil
}

// packOne packs one source folder and returns the lines to print.
func packOne(dir, dist string) (string, error) {
	p, err := loadSource(dir)
	if err != nil {
		return "", err
	}
	data, err := csw.Encode(p)
	if err != nil {
		return "", err
	}
	target := filepath.Join(dist, filepath.FromSlash(csw.VersionPath(p.ID, p.Version)))
	status := "packed   "
	if old, err := os.ReadFile(target); err == nil { //nolint:gosec // inside the caller's dist folder
		if !bytes.Equal(old, data) {
			return "", fmt.Errorf("%s exists with different content (published versions are immutable; bump the version in %s)", target, csw.ManifestFile)
		}
		status = "unchanged"
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read %s: %w", target, err)
	}
	if status == "packed   " {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec // published files are public
			return "", fmt.Errorf("create %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil { //nolint:gosec // published files are public
			return "", fmt.Errorf("write %s: %w", target, err)
		}
	}
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%s %s %s (%d bytes)\n", status, p.ID, p.Version, len(data))
	for _, w := range p.Warnings() {
		_, _ = fmt.Fprintf(&b, "warn %s: %s\n", p.ID, w)
	}
	return b.String(), nil
}

// loadSource reads and validates a source folder, whose name must be the
// widget's ID.
func loadSource(dir string) (*csw.Package, error) {
	p, err := csw.FromDir(dir)
	if err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if base := filepath.Base(filepath.Clean(dir)); base != p.ID {
		return nil, fmt.Errorf("the folder is named %q but the widget's id is %q", base, p.ID)
	}
	return p, nil
}

// sourceFolders resolves root to the source folders it names: itself if it
// holds a manifest.json, else its subfolders (skipping dotted ones).
func sourceFolders(root string) ([]string, error) {
	if _, err := os.Stat(filepath.Join(root, csw.ManifestFile)); err == nil { //nolint:gosec // the caller names the folder
		return []string{root}, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", root, err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("%s holds no widget folders", root)
	}
	return dirs, nil
}

// check validates source folders or one .csw file and prints the result.
func check(args []string, stdout, stderr io.Writer) error {
	flags := newFlags("check", "<src-root|folder|file.csw>", stderr)
	pos, err := parse(flags, args)
	if err != nil {
		return err
	}
	targets := []string{pos[0]}
	if st, err := os.Stat(pos[0]); err != nil { //nolint:gosec // the caller names the path
		return fmt.Errorf("check: %w", err)
	} else if st.IsDir() {
		if targets, err = sourceFolders(pos[0]); err != nil {
			return err
		}
	}
	failed := 0
	for _, t := range targets {
		p, err := loadTarget(t)
		if err != nil {
			failed++
			_, _ = fmt.Fprintf(stdout, "FAIL %s: %v\n", t, err)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "ok   %s %s\n", p.ID, p.Version)
		for _, w := range p.Warnings() {
			_, _ = fmt.Fprintf(stdout, "warn %s: %s\n", p.ID, w)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%w: %d of %d", errFailed, failed, len(targets))
	}
	return nil
}

// loadTarget loads a source folder or .csw file and validates it.
func loadTarget(target string) (*csw.Package, error) {
	st, err := os.Stat(target) //nolint:gosec // the caller names the path
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return loadSource(target)
	}
	f, err := os.Open(target) //nolint:gosec // the caller names the file
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	p, err := csw.Decode(f)
	if err != nil {
		return nil, err
	}
	return p, p.Validate()
}

// index writes and signs the dist folder's index.
func index(args []string, stdout, stderr io.Writer) error {
	flags := newFlags("index", "<dist> -key <private key> [-featured id,id] [-now RFC3339]", stderr)
	keyPath := flags.String("key", "", "private signing key `file` (as made by csw genkey)")
	featured := flags.String("featured", "", "comma-separated widget `ids` to mark featured")
	now := flags.String("now", "", "the publish `time` for new versions (RFC3339; default now)")
	pos, err := parse(flags, args)
	if err != nil {
		return err
	}
	if err := required(flags, "key"); err != nil {
		return err
	}
	when := time.Now()
	if *now != "" {
		if when, err = time.Parse(time.RFC3339, *now); err != nil {
			return fmt.Errorf("-now: %w", err)
		}
	}
	raw, err := os.ReadFile(*keyPath)
	if err != nil {
		return fmt.Errorf("read key: %w", err)
	}
	priv, err := csw.ParsePrivateKey(raw)
	if err != nil {
		return err
	}
	prev, err := csw.ReadIndex(pos[0])
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("previous index: %w", err)
	}
	opts := csw.IndexOptions{Now: when, Previous: prev}
	if *featured != "" {
		opts.Featured = slices.DeleteFunc(strings.Split(*featured, ","), func(s string) bool { return strings.TrimSpace(s) == "" })
	}
	ix, err := csw.BuildIndex(pos[0], opts)
	if err != nil {
		return err
	}
	if _, err := csw.WriteIndex(pos[0], ix, priv); err != nil {
		return err
	}
	versions := 0
	for _, w := range ix.Widgets {
		versions += len(w.Versions)
	}
	_, _ = fmt.Fprintf(stdout, "indexed %d widgets (%d versions); wrote %s and %s\n", len(ix.Widgets), versions, csw.IndexFile, csw.IndexSigFile)
	return nil
}

// verify checks the signed index and the files it lists.
func verify(args []string, stdout, stderr io.Writer) error {
	flags := newFlags("verify", "<dist> -pub <public key>", stderr)
	pubArg := flags.String("pub", "", "public key `file` (or its base64)")
	pos, err := parse(flags, args)
	if err != nil {
		return err
	}
	if err := required(flags, "pub"); err != nil {
		return err
	}
	pub, err := readPublicKey(*pubArg)
	if err != nil {
		return err
	}
	n, problems := csw.VerifyDist(pos[0], pub)
	for _, p := range problems {
		_, _ = fmt.Fprintf(stdout, "FAIL %v\n", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %d problems", errFailed, len(problems))
	}
	_, _ = fmt.Fprintf(stdout, "ok   index signature and %d files\n", n)
	return nil
}

// readPublicKey reads a public key file, or takes the argument as base64.
func readPublicKey(arg string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(filepath.Clean(arg))
	if err != nil {
		data = []byte(arg)
	}
	return csw.ParsePublicKey(data)
}

// genkey writes a new keypair.
func genkey(args []string, stdout, stderr io.Writer) error {
	flags := newFlags("genkey", "-out <file> [-force]", stderr)
	out := flags.String("out", "", "private key `file`; the public key goes to the same path + .pub")
	force := flags.Bool("force", false, "overwrite existing key files")
	if err := noArgs(flags, args); err != nil {
		return err
	}
	if err := required(flags, "out"); err != nil {
		return err
	}
	pubPath := *out + ".pub"
	if !*force {
		for _, p := range []string{*out, pubPath} {
			if _, err := os.Stat(p); err == nil {
				return fmt.Errorf("%s exists (use -force to replace it; galleries signed with the old key could then no longer be verified by it)", p)
			}
		}
	}
	priv, pub, err := csw.GenerateKey(nil)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(*out); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("key dir: %w", err)
		}
	}
	if err := os.WriteFile(*out, priv, 0o600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}
	// WriteFile keeps an existing file's mode; -force must not leave a
	// readable private key behind.
	if err := os.Chmod(*out, 0o600); err != nil {
		return fmt.Errorf("private key mode: %w", err)
	}
	if err := os.WriteFile(pubPath, pub, 0o644); err != nil { //nolint:gosec // a public key
		return fmt.Errorf("write public key: %w", err)
	}
	_, _ = fmt.Fprintf(stdout, "private key: %s (keep it secret and backed up; never commit it)\n", *out)
	_, _ = fmt.Fprintf(stdout, "public key:  %s\n", pubPath)
	return nil
}
