package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

const inventoryDir = `/inventory`

type Inventory struct {
	Dir          string
	fs           afero.Fs
	clusterNames []string
}

type Cluster struct {
	Name      string
	inventory *ClusterInventory
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

type YamlHosts map[string]map[string]string // "hostname":{"var1":"val"}

type InventoryHosts struct { // hosts:{"hostname":{"var1":"val"}}
	Hosts YamlHosts `yaml:"hosts"`
}

type YamlHostGroup map[string]InventoryHosts // "gluster":{hosts:{"hostname":{}}}

type YamlAll struct { // hosts:{"hostname":{"var1":"val"}},children:{"gluster":{hosts:{"hostname":{}}}}
	Hosts  YamlHosts     `yaml:"hosts"`
	Groups YamlHostGroup `yaml:"children"`
}

type ClusterInventory struct { // all:hosts:{"hostname":{"var1":"val"}},children:{"gluster":{"hostname":{}}}
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
	return fmt.Sprintf("Inventory at %q has %d clusters defined.\n", inv.Dir, len(inv.clusterNames))
}

// func (inv *Inventory) LoadCluster(name string) (Cluster, error) {
// }

func (inv *Inventory) NewCluster(name string, glusterHosts []string) (Cluster, error) {
	if inv.ContainsCluster(name) {
		return Cluster{}, errors.New(fmt.Sprintf("Cluster named %q already in the inventory.", name))
	}

	clusterInv := NewClusterInventory(name, glusterHosts)
	yaml, err := clusterInv.toYaml()
	if err != nil {
		return Cluster{}, err
	}

	// Write the YAML inventory file
	if err := inv.writeYamlFile(name, yaml); err != nil {
		return Cluster{}, errors.New(fmt.Sprintf("Unable to write YAML inventory: %s\n", err))
	}

	inv.addClusterName(name)

	return Cluster{name, clusterInv}, nil
}

func (inv *Inventory) addClusterName(name string) {
	inv.clusterNames = append(inv.clusterNames, name)
}

func (inv *Inventory) ContainsCluster(name string) bool {
	for _, v := range inv.clusterNames {
		if v == name {
			return true
		}
	}
	return false
}

func (inv *Inventory) writeYamlFile(name string, data []byte) error {
	path := inventoryDir + fmt.Sprintf("/%s.yml", name)
	afs := &afero.Afero{Fs: inv.fs}
	return afs.WriteFile(path, data, 0644)
}

func NewClusterInventory(name string, glusterHosts []string) *ClusterInventory {
	// Assemble the inventory. For now, we're ignoring the variables
	hosts := YamlHosts{}
	vars := make(map[string]string)
	for _, v := range glusterHosts {
		hosts[v] = vars
	}
	group := YamlHostGroup{"gluster": InventoryHosts{hosts}}

	return &ClusterInventory{YamlAll{hosts, group}}
}

func (inv *ClusterInventory) toYaml() ([]byte, error) {
	yaml, err := yaml.Marshal(inv)
	if err != nil {
		return yaml, errors.New(fmt.Sprintf("Unable to generate YAML inventory: %s\n", err))
	}

	return yaml, err
}
