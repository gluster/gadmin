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

	// If $GADMIN_HOME must be an absolute path.
	// Ref: https://github.com/gluster/gadmin/issues/6#issuecomment-399989073
	if !filepath.IsAbs(gadminHome) {
		fmt.Println("$GADMIN_HOME is not an absolute path; exiting.")
		os.Exit(254)
	}

	// The directory needs to exist, be a directory and be writable.
	switch exists, err := afero.DirExists(new(afero.OsFs), gadminHome); {
	case !exists:
		fmt.Printf("$GADMIN_PATH %q doesn't exist or is not a directory.\n", gadminHome)
		os.Exit(254)
	case err != nil:
		fmt.Printf("Error while checking for $GADMIN_PATH %q: %v", gadminHome, err)
		os.Exit(254)
	}

	if unix.Access(gadminHome, unix.W_OK) != nil {
		fmt.Printf("$GADMIN_PATH %q is not writable; exiting.\n", gadminHome)
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
