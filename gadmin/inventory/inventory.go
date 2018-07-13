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

type InventoryCluster struct {
	name      string
	inventory *YamlInventory
}

// Ansible YAML inventory structure
// Ex.
// all:
//   hosts:
//     192.168.100.71:
//       var1: foo
//     192.168.100.72:
//       var1: foo
//       var2: bar
//     192.168.100.73:
//   children:
//     gluster:
//       hosts:
//         192.168.100.71:
//         192.168.100.72:
//         192.168.100.73:

type InventoryHosts map[string]map[string]string // "hostname":{"var1":"val"}

type YamlHosts struct { // hosts:{"hostname":{"var1":"val"}}
	Hosts InventoryHosts `yaml:"hosts"`
}

type YamlHostGroup map[string]YamlHosts // "gluster":{hosts:{"hostname":{}}}

type YamlAll struct { // hosts:{"hostname":{"var1":"val"}},children:{"gluster":{hosts:{"hostname":{}}}}
	Hosts  InventoryHosts `yaml:"hosts"`
	Groups YamlHostGroup  `yaml:"children"`
}

type YamlInventory struct { // all:hosts:{"hostname":{"var1":"val"}},children:{"gluster":{"hostname":{}}}
	All YamlAll
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
