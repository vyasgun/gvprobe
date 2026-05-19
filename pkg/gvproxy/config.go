package gvproxy

import (
	"fmt"
	"os"

	gvproxytypes "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/vyasgun/gvprobe/pkg/constants"
)

func NewDefaulCommand() gvproxytypes.GvproxyCommand {
	return NewCommand("vfkit")
}

// NewCommand builds a gvproxy command with one datapath listen socket for backend.
// Supported backends: vfkit (default), bess.
func NewCommand(backend string) gvproxytypes.GvproxyCommand {
	if backend == "" {
		backend = "vfkit"
	}
	cmd := gvproxytypes.NewGvproxyCommand()
	cmd.PidFile = constants.GvproxyPidFile
	cmd.LogFile = constants.GvproxyLogFile
	cmd.Debug = true
	cmd.SSHPort = constants.GvproxySSHPort
	switch backend {
	case "bess":
		cmd.AddBessSocket(fmt.Sprintf("unixpacket://%s", constants.BessSocket))
	default:
		cmd.AddVfkitSocket(fmt.Sprintf("unixgram://%s", constants.VfkitSocket))
	}
	cmd.AddServiceEndpoint(constants.GvproxyServicesListen)
	return cmd
}

func RunCommand(cmd gvproxytypes.GvproxyCommand) (int, error) {
	runCmd := cmd.Cmd("gvproxy")
	runCmd.Env = os.Environ()
	if err := runCmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start gvproxy: %v", err)
	}
	return runCmd.Process.Pid, nil
}
