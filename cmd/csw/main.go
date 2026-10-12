// Command csw packs, checks and signs .csw widget packages: it validates
// widget source folders, packs them as .csw files, and writes and verifies
// the signed gallery index. The logic is in internal/cli; run it with no
// arguments for the commands.
package main

import (
	"os"

	"github.com/crostream/csw/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
