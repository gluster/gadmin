package ansrunner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gluster/gadmin/gadmin/inventory"
	"golang.org/x/sys/unix"
)

var runnerInputDirs = []string{`inventory`, `project`}

var runnerExePath string

func init() {
	// Check if ansible-runner is in $PATH and is executable.
	path, err := exec.LookPath("ansible-runner")

	if err != nil {
		fmt.Println("ansible-runner not found in $PATH: %q", os.Getenv("PATH"))
		os.Exit(253)
	}

	if err := ensureFileExecutable(path); err != nil {
		fmt.Println(err)
		os.Exit(253)
	}

	runnerExePath = path
}

type AnsRunner struct {
	BaseDir       string
	cluster       *inventory.Cluster
	Exe           string
	Invocation    *exec.Cmd
	hosts         []string
	overrideHosts bool
	args          AnsRunnerArgs
}

type AnsRunnerArgs struct {
	Playbook string
	Ident    string
}

func New(baseDir string, cluster *inventory.Cluster, id string) (*AnsRunner, error) {
	path, err := getAbsPath(baseDir)
	if err != nil {
		return nil, err
	}

	runner := AnsRunner{}
	runner.BaseDir = path
	runner.cluster = cluster
	runner.Exe = runnerExePath
	runner.args.Ident = id
	return &runner, nil
}

func (ans *AnsRunner) SetTarget(targettype string, targets []string) error {
	if len(targets) == 0 {
		return errors.New("No targets provided.")
	}

	switch {
	case targettype == `group`:
		if err := ans.setHostsFromGroups(targets); err != nil {
			return err
		}
	case targettype == `hosts`:
		if err := ans.setHosts(targets); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Target type must be either \"hosts\" or \"group\". Supplied %q.", targettype)
	}
	return nil
}

func (ans *AnsRunner) setHosts(hosts []string) error {
	ans.hosts = hosts
	ans.overrideHosts = true
	return nil
}

func (ans *AnsRunner) setHostsFromGroups(groups []string) error {
	// Get the combined list of hosts from all the groups
	hostsInGroups := ans.cluster.Inventory.HostsInGroups(groups)

	// If there are no hosts, return an error
	if len(hostsInGroups) == 0 {
		return errors.New("No hosts found in the group(s) provided.")
	}

	// If hosts are found, setup the ans.hosts list
	ans.hosts = make([]string, len(hostsInGroups))
	i := 0
	for host := range hostsInGroups {
		ans.hosts[i] = host
	}

	// Also set the override so that the hosts list is provided explicitly
	// during ansible-runner invocation
	ans.overrideHosts = true

	return nil
}

func (ans *AnsRunner) CreateBaseDir() error {
	if err := os.Mkdir(ans.BaseDir, 0755); err != nil {
		return fmt.Errorf("Unable to create the runner base directory %q: %v", ans.BaseDir, err)
	}

	return nil
}

func (ans *AnsRunner) SetupInputDirs() error {
	path := ans.BaseDir

	if err := ensureDirWritable(path); err != nil {
		return err
	}

	for _, dirName := range runnerInputDirs {
		inputDir := filepath.Join(path, dirName)
		if err := os.Mkdir(inputDir, 0755); err != nil {
			return fmt.Errorf("Unable to create runner input directory %q: %v", inputDir, err)
		}
	}

	return nil
}

func (ans *AnsRunner) AddPlaybookFile(file string) error {
	src, err := getAbsPath(filepath.Clean(file))
	if err != nil {
		return fmt.Errorf("Unable to get absolute path for the playbook source file %q: %v", file, err)
	}
	if err := ensureFileReadable(src); err != nil {
		return err
	}

	invDir := filepath.Join(ans.BaseDir, `inventory`)
	if err := ensureDirWritable(invDir); err != nil {
		return err
	}

	dst := filepath.Join(invDir, filepath.Base(file))

	if err = copyFile(src, dst); err != nil {
		return fmt.Errorf("Failed to copy playbook from %q to %q: %v", src, dst, err)
	}

	ans.args.Playbook = dst

	return nil
}

func (ans *AnsRunner) SetupInvocation() {
	ans.Invocation = exec.Command("ansible-runner", "-p", ans.args.Playbook, "-i", ans.args.Ident, "run", ans.BaseDir)
}

func getAbsPath(path string) (string, error) {
	path = filepath.Clean(path)

	if !filepath.IsAbs(path) {
		var err error
		if path, err = filepath.Abs(path); err != nil {
			return "", fmt.Errorf("Could not derive absolute path for %q: %v", path, err)
		}
	}

	return path, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

func ensureDirWritable(path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	if err := unix.Access(path, unix.W_OK); err != nil {
		return fmt.Errorf("Directory %q is not writable.", path)
	}

	return nil
}

func ensureDir(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Unable to access %q: %v", path, err)
	}

	mode := stat.Mode()
	if !mode.IsDir() {
		return fmt.Errorf("%q is not a directory.", path)
	}

	return nil
}

func ensureFileReadable(path string) error {
	if err := ensureFile(path); err != nil {
		return err
	}

	if err := unix.Access(path, unix.R_OK); err != nil {
		return fmt.Errorf("File %q is not readable.", path)
	}

	return nil
}

func ensureFileExecutable(path string) error {
	if err := ensureFile(path); err != nil {
		return err
	}

	if err := unix.Access(path, unix.X_OK); err != nil {
		return fmt.Errorf("File %q is not executable.", path)
	}

	return nil
}

func ensureFile(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Unable to access %q: %v", path, err)
	}

	mode := stat.Mode()
	if !mode.IsRegular() {
		return fmt.Errorf("%q is not a file.", path)
	}

	return nil
}
