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

// This is the type to be worked with to run a playbook via ansible-runner. It
// is possible to run multiple playbooks via the same instance of AnsRunner.
type AnsRunner struct {
	BaseDir       string
	cluster       *inventory.Cluster
	Exe           string
	hosts         []string
	overrideHosts bool
	Invocation    AnsRunnerInvocation
}

// The AnsRunnerArgs get setup internally by New() and
// AnsRunner.AddPlaybook(). The interface is provided to enable test cases to
// modify the arguments for multiple runs using the same AnsRunner instance,
// without having to do the setup and teardown each time. In actual code, this
// type shouldn't need to be used directly unless to log the details for
// debugging.
type AnsRunnerInvocation struct {
	Ident    string
	Playbook string
	Cmd      *exec.Cmd
	Setup    bool
}

// First create a new instance using New(). The BaseDir is the directory under
// which the ansible-runner directory structure is expected to exit. This is
// also the directory to which the output artifacts will be written. cluster
// being a pointer, allows the inventory to be refreshed and the runner to be
// run against the refreshed inventory.
func New(baseDir string, cluster *inventory.Cluster) (*AnsRunner,
	error) {
	path, err := getAbsPath(baseDir)
	if err != nil {
		return nil, err
	}

	runner := AnsRunner{}
	runner.BaseDir = path
	runner.cluster = cluster
	runner.Exe = runnerExePath
	return &runner, nil
}

func (ans *AnsRunner) Execute() error {
	// TODO: Don't need to throw out internal errors from the likes of ans.run()
	// to the user of this type. Might be better to setup proper error types.
	err := ans.run()
	if err != nil {
		return err
	}
	return nil
}

// Private method to check whether the invocation has been setup before
// running the command. No need to expose the invocation details to the user
// of this type.
func (ans *AnsRunner) run() error {
	if !ans.Invocation.Setup {
		return errors.New("Invocation not setup. Call *AnsRunner.SetupInvocation() first.")
	}
	return ans.Invocation.Cmd.Run()
}

// Call this *only to* override the execution of a playbook against a specific set of
// hosts. This will setup a list of hosts or all the hosts from a target group
// to run the play against by specifying them directly on the command line to
// ansible-runner.
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

// This will create the top level base directory in which all ansible-runner
// input and output will reside. This is required only once before the first
// invocation of an AnsRunner instance. Every invocation will use the same
// directory thereafter.
func (ans *AnsRunner) CreateBaseDir() error {
	if err := os.Mkdir(ans.BaseDir, 0755); err != nil {
		return fmt.Errorf("Unable to create the runner base directory %q: %v", ans.BaseDir, err)
	}

	return nil
}

// If the base directory has already been created, call to create the input
// directories inside it. As with CreateBaseDir(), only requires to be called
// once before the first invocation of an AnsRunner instance.
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

// Needs to be called to setup an invocation for a specific playbook to be
// run. This means that it must be called at least once before
// AnsRunner.Execute() can be called. Can be called multiple times to replace
// the AnsRunner.Invocation value to run multiple plays against the same
// runner base directory.
func (ans *AnsRunner) SetupInvocation(ident string, playbookFile string) error {
	playbook, err := ans.addPlaybookFile(playbookFile)
	if err != nil {
		return err
	}

	cmd := exec.Command("ansible-runner", "-p", playbook, "-i", ident, "run", ans.BaseDir)

	ans.Invocation = AnsRunnerInvocation{ident, playbook, cmd, true}

	return nil
}

// Copies a playbook file to the input directory.
func (ans *AnsRunner) addPlaybookFile(file string) (string, error) {
	var dst string

	src, err := getAbsPath(filepath.Clean(file))
	if err != nil {
		return dst, fmt.Errorf("Unable to get absolute path for the playbook source file %q: %v", file, err)
	}
	// No need for readable check, os.Open() will fail in copyFile().
	//If err := ensureFileReadable(src); err != nil {
	//	return "", err
	//}

	invDir := filepath.Join(ans.BaseDir, `inventory`)
	if err := ensureDirWritable(invDir); err != nil {
		return dst, err
	}

	dst = filepath.Join(invDir, filepath.Base(file))

	if err = copyFile(src, dst); err != nil {
		return dst, fmt.Errorf("Failed to copy playbook from %q to %q: %v", src, dst, err)
	}

	return dst, nil
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
