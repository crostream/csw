// Package cli is the logic of the csw command: the tooling that turns
// widget source folders into .csw files and the signed files of a widget
// gallery (see package csw).
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

const usage = `usage: csw <command> [flags]

commands:
  pack <src-root> -o <dist>             validate source folders and write dist/w/<id>/<version>.csw
  check <src-root|folder|file.csw>      validate sources or a .csw file
  index <dist> -key <private key>       write and sign dist/index.json
        [-featured id,id] [-now RFC3339]
  verify <dist> -pub <public key>       check the signed index and every file it lists
  genkey -out <file> [-force]           write a new signing keypair (<file> and <file>.pub)

Run "csw <command> -h" for a command's flags.
`

var (
	// errUsage reports bad arguments; the message has been printed.
	errUsage = errors.New("usage")
	// errFailed reports that some items failed; each was printed.
	errFailed = errors.New("some widgets failed")
)

// Run runs the command in args and returns the process exit code: 0 on
// success, 1 on a failure, 2 on bad usage.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "pack":
		err = pack(args[1:], stdout, stderr)
	case "check":
		err = check(args[1:], stdout, stderr)
	case "index":
		err = index(args[1:], stdout, stderr)
	case "verify":
		err = verify(args[1:], stdout, stderr)
	case "genkey":
		err = genkey(args[1:], stdout, stderr)
	case "-h", "-help", "--help", "help":
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "csw: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.Is(err, errUsage):
		return 2
	}
	_, _ = fmt.Fprintf(stderr, "csw %s: %v\n", args[0], err)
	return 1
}

// newFlags returns a flag set printing to stderr.
func newFlags(name, synopsis string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "usage: csw %s %s\n", name, synopsis)
		fs.PrintDefaults()
	}
	return fs
}

// parse parses args with flags allowed before, between and after the
// positional arguments, and checks there is exactly one of them.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, err
			}
			return nil, errUsage
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(pos) != 1 {
		_, _ = fmt.Fprintf(fs.Output(), "expected one argument, got %d\n", len(pos))
		fs.Usage()
		return nil, errUsage
	}
	return pos, nil
}

// noArgs parses args, which must hold flags only.
func noArgs(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return errUsage
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(fs.Output(), "unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return errUsage
	}
	return nil
}

// required reports the first empty flag among names.
func required(fs *flag.FlagSet, names ...string) error {
	for _, n := range names {
		if fs.Lookup(n).Value.String() == "" {
			_, _ = fmt.Fprintf(fs.Output(), "-%s is required\n", n)
			fs.Usage()
			return errUsage
		}
	}
	return nil
}
