package gadmin

import (
	"fmt"

	"github.com/gluster/gadmin/gadmin/inventory"
	"github.com/spf13/afero"
)

type Gadmin struct {
	Home      string
	WorkFs    afero.Fs
	Inventory *inventory.Inventory
}

func New(homeDirPath string) (*Gadmin, error) {
	gadmin, err := initGadm(afero.NewOsFs(), homeDirPath)
	return gadmin, err
}

func initGadm(fs afero.Fs, path string) (*Gadmin, error) {
	// Here's why we use afero. All operations will be restricted to this
	// directory hereonwards.
	fs = afero.NewBasePathFs(afero.NewOsFs(), path)

	inventory, err := inventory.New(path)
	if err != nil {
		return &Gadmin{}, err
	}

	return &Gadmin{path, fs, inventory}, nil
}

func (gadm Gadmin) String() string {
	return fmt.Sprintf("%q running against %q.\n%s", `gadmin`, gadm.Home, gadm.Inventory)
}
