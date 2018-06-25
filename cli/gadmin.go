package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"golang.org/x/sys/unix"
)

var GadminHome *afero.BasePathFs // All the runtime files; except logs; will be written here.

// Startup checks
func init() {
	// We shouldn't be running as root.
	if os.Geteuid() == 0 {
		fmt.Println("Running as root is not supported; exiting.")
		os.Exit(254)
	}

	// $GADMIN_HOME needs to be defined.
	gadminHome := os.Getenv("GADMIN_HOME")

	if gadminHome == "" {
		fmt.Println("$GADMIN_HOME not set; exiting.")
		os.Exit(254)
	}

	// If $GADMIN_HOME isn't an absolute path, turn it into to ensure that no
	// matter where we're run from, we always write to the same directory.
	if !filepath.IsAbs(gadminHome) {
		fmt.Printf("$GADMIN_HOME is not an absolute path; ")
		// Not sure what filepath.Abs returns the error for, but since it does, we
		// use it.
		var err error
		if gadminHome, err = filepath.Abs(gadminHome); err != nil {
			fmt.Printf("error deriving absolute path: %v", err)
			os.Exit(254)
		}

		fmt.Printf("using %q.\n", gadminHome)
	}

	// The directory needs to exist, be a directory and be writable.
	switch exists, err := afero.DirExists(new(afero.OsFs), gadminHome); {
	case !exists:
		fmt.Printf("%q doesn't exist or is not a directory.\n", gadminHome)
		os.Exit(254)
	case err != nil:
		fmt.Printf("Error while checking for directory %q: %v", gadminHome, err)
		os.Exit(254)
	}

	if unix.Access(gadminHome, unix.W_OK) != nil {
		fmt.Printf("%q is not writable; exiting.\n", gadminHome)
		os.Exit(254)
	}

	// Here's why we use afero. All operations will be restricted to this
	// directory hereonwards.
	GadminHome = afero.NewBasePathFs(afero.NewOsFs(), gadminHome).(*afero.BasePathFs)
	// So RealPath() returns an error if the directory is outside the base path.
	if path, err := GadminHome.RealPath("/"); err != nil {
		fmt.Printf("Error: %q.\nThis is a bug.\n", err)
		os.Exit(254)
	} else {
		fmt.Printf("Using %q as the work directory.\n", path)
	}
}

func main() {
	fmt.Println("gadmin")
}
