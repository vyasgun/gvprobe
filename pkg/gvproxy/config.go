package gvproxy

import (
	"fmt"
	"os"

	gvproxytypes "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/vyasgun/gvprobe/pkg/constants"
)

func NewDefaulCommand() gvproxytypes.GvproxyCommand {
	cmd := gvproxytypes.NewGvproxyCommand()
	cmd.PidFile = constants.GvproxyPidFile
	cmd.LogFile = constants.GvproxyLogFile
	cmd.Debug = true
	cmd.AddVfkitSocket(fmt.Sprintf("unixgram://%s", constants.VfkitSocket))
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
