package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	//"gopkg.in/yaml.v2"
)

const inventoryDir = `/inventory`

type Inventory struct {
	Dir          string
	fs           afero.Fs
	ClusterNames []string
}

type Cluster struct {
	GlusterHosts *[]Host
}

type Host struct {
	Name string
	Vars map[string]string
}

func New(path string) (*Inventory, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return &Inventory{}, err
	}

	if !strings.HasSuffix(path, inventoryDir) {
		abs = filepath.Join(abs, inventoryDir)
	}

	inv, err := initInv(afero.NewOsFs(), abs)
	return inv, err
}

func initInv(fs afero.Fs, path string) (*Inventory, error) {
	var clusterNames []string
	fs = afero.NewBasePathFs(fs, path)

	if exists, err := afero.DirExists(fs, path); err != nil {
		return &Inventory{}, err
	} else if !exists {
		return &Inventory{path, fs, clusterNames}, nil
	}

	var files []os.FileInfo
	afs := afero.Afero{Fs: fs}
	var err error
	if files, err = afs.ReadDir(path); err != nil {
		return &Inventory{}, err
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), `.yml`) {
			cluster := strings.TrimSuffix(f.Name(), `.yml`)
			clusterNames = append(clusterNames, cluster)
		}
	}

	return &Inventory{path, fs, clusterNames}, nil
}

func (inv Inventory) String() string {
	return fmt.Sprintf("Inventory at %q has %d clusters defined.\n", inv.Dir, len(inv.ClusterNames))
}

// func (inv *Inventory) LoadCluster(name string) (Cluster, error) {
// }
