package inventory

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/sys/unix"
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
	Inventory *ClusterInventory
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
	path = filepath.Clean(path)

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

func (inv *Inventory) LoadCluster(name string) (Cluster, error) {
	if !inv.ContainsCluster(name) {
		return Cluster{}, errors.New(fmt.Sprintf("Cluster named %q isn't in the inventory.", name))
	}

	var yamlData []byte
	var err error

	if yamlData, err = inv.readInventoryFile(name); err != nil {
		return Cluster{}, errors.New(fmt.Sprintf("Unable load inventory file for cluster %q: %s\n", name, err))
	}

	clusterInv := &ClusterInventory{}
	if err = clusterInv.fromYaml(yamlData); err != nil {
		return Cluster{}, errors.New(fmt.Sprintf("Unable to load cluster from inventory %q: %s\n", name, err))
	}

	inv.addClusterName(name)

	return Cluster{name, clusterInv}, nil
}

func (inv *Inventory) NewCluster(name string, glusterHosts []string) (Cluster, error) {
	if inv.ContainsCluster(name) {
		return Cluster{}, errors.New(fmt.Sprintf("Cluster named %q already in the inventory.", name))
	}

	clusterInv := NewClusterInventory(name, glusterHosts)
	yamlFile, err := clusterInv.toYaml()
	if err != nil {
		return Cluster{}, err
	}

	// Write the YAML inventory file
	if err := inv.writeInventoryFile(name, yamlFile); err != nil {
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

func (inv *Inventory) readInventoryFile(name string) ([]byte, error) {
	relPath := inventoryFileRelPath(name)
	afs := &afero.Afero{Fs: inv.fs}
	return afs.ReadFile(relPath)
}

func (inv *Inventory) writeInventoryFile(name string, data []byte) error {
	relPath := inventoryFileRelPath(name)
	afs := &afero.Afero{Fs: inv.fs}
	return afs.WriteFile(relPath, data, 0644)
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

func (inv *ClusterInventory) GroupNames() []string {
	names := make([]string, len(inv.All.Groups)+1)
	names[0] = `all`
	i := 1
	for group := range inv.All.Groups {
		names[i] = group
	}

	return names
}

func (inv *ClusterInventory) HasGroup(group string) bool {
	_, found := inv.All.Groups[group]
	return found
}

func (inv *ClusterInventory) HostsInGroups(groups []string) map[string][]string {
	hosts := make(map[string][]string)

	// Go over each group
	for _, group := range groups {
		switch {
		case group == `all`:
			// If `all` is specified as a group, set the list of all hosts that aren't
			// already in the list
			for _, host := range inv.AllHosts() {
				if _, found := hosts[host]; !found {
					hosts[host] = []string{}
				}
			}
		case !inv.HasGroup(group):
			// Skip silently if the group doesn't exist
			continue
		default:
			// If the group does exist, get its list of hosts and go over each
			for _, host := range inv.GroupHosts(group) {
				if _, found := hosts[host]; found {
					// If the host already exists, add the group to its list
					hosts[host] = append(hosts[host], group)
				} else {
					// otherwise, create a new host entry
					hosts[host] = []string{group}
				}
			}
		}
	}

	return hosts
}

func (inv *ClusterInventory) AllHosts() []string {
	hosts := make([]string, len(inv.All.Hosts))
	i := 0
	for host := range inv.All.Hosts {
		hosts[i] = host
	}

	return hosts
}

func (inv *ClusterInventory) GroupHosts(group string) []string {
	var hosts []string
	if inv.HasGroup(group) {
		hosts = inv.All.Groups[group].HostNames()
	}
	return hosts
}

func (inv *ClusterInventory) toYaml() ([]byte, error) {
	yamlOut, err := yaml.Marshal(inv)
	if err != nil {
		return yamlOut, errors.New(fmt.Sprintf("Unable to generate YAML inventory: %s\n", err))
	}

	return yamlOut, err
}

func (inv *ClusterInventory) fromYaml(yamlData []byte) error {
	if err := yaml.Unmarshal(yamlData, inv); err != nil {
		return err
	}
	return nil
}

func (invHosts InventoryHosts) HostNames() []string {
	hosts := make([]string, len(invHosts.Hosts))
	i := 0
	for host := range invHosts.Hosts {
		hosts[i] = host
	}
	return hosts
}

func WriteYamlInventoryToDir(cluster *Cluster, dir string) error {
	path := filepath.Clean(dir)

	if !filepath.IsAbs(dir) {
		var err error
		if path, err = filepath.Abs(dir); err != nil {
			return fmt.Errorf("Could not derive the absolute path for %q: %v", dir, err)
		}
	}

	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Unable to access %q: %v", path, err)
	}

	mode := stat.Mode()
	if !mode.IsDir() {
		return fmt.Errorf("%q is not a directory.", path)
	}

	if err := unix.Access(path, unix.W_OK); err != nil {
		return fmt.Errorf("Directory %q is not writable: %v", dir, err)
	}

	path = filepath.Join(path, cluster.Name, `.yml`)

	yaml, err := cluster.Inventory.toYaml()
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(path, yaml, 0644); err != nil {
		return fmt.Errorf("Unable to write yaml file %q: %v", path, err)
	}

	return nil
}

func inventoryFileRelPath(name string) string {
	return inventoryDir + fmt.Sprintf("%s.yml", name)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
