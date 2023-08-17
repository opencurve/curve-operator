package exec

import (
	"fmt"
	"k8s.io/klog"
	"os/exec"
	"strings"
)

// Executor is the main interface for all the exec commands
type Executor interface {
	ExecuteCommandWithOutput(command string, arg ...string) (string, error)
}

// CommandExecutor is the type of the Executor
type CommandExecutor struct{}

// ExecuteCommandWithOutput executes a command with output
func (*CommandExecutor) ExecuteCommandWithOutput(command string, arg ...string) (string, error) {
	logCommand(command, arg...)
	cmd := exec.Command(command, arg...)
	return runCommandWithOutput(cmd, false)
}

func runCommandWithOutput(cmd *exec.Cmd, combinedOutput bool) (string, error) {
	var output []byte
	var err error
	var out string

	if combinedOutput {
		output, err = cmd.CombinedOutput()
	} else {
		output, err = cmd.Output()
		if err != nil {
			output = []byte(fmt.Sprintf("%s. %s", string(output), assertErrorType(err)))
		}
	}

	out = strings.TrimSpace(string(output))

	if err != nil {
		return out, err
	}

	return out, nil
}

func logCommand(command string, arg ...string) {
	klog.Infof("Running command: %s %s", command, strings.Join(arg, " "))
}

func assertErrorType(err error) string {
	switch errType := err.(type) {
	case *exec.ExitError:
		return string(errType.Stderr)
	case *exec.Error:
		return errType.Error()
	}

	return ""
}
