package attach

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/vyasgun/gvprobe/pkg/constants"
	"github.com/vyasgun/gvprobe/pkg/gvproxy"
	"github.com/vyasgun/gvprobe/pkg/gvproxy/datapath"
)

// Run holds a persistent datapath session open until ctx is cancelled.
// It also serves a proxy on attach.sock so trace commands can forward
// frames through the held session instead of dialing gvproxy directly.
func Run(ctx context.Context, backend string) error {
	status, err := gvproxy.Status()
	if err != nil {
		return err
	}
	if status != gvproxy.Running {
		return fmt.Errorf("gvproxy is not running (run 'gvprobe start' first)")
	}
	if err := ensureNotAttached(); err != nil {
		return err
	}
	if err := writeAttachPID(); err != nil {
		return err
	}
	defer cleanupAttach()

	return datapath.WithSession(backend, func(s datapath.Session) error {
		_ = os.Remove(constants.AttachSocket)
		ln, err := net.Listen("unix", constants.AttachSocket)
		if err != nil {
			return fmt.Errorf("attach proxy listen: %w", err)
		}
		defer ln.Close()

		var mu sync.Mutex
		go serveProxy(ctx, ln, &mu, s)

		log.Printf("attached (backend=%s), Ctrl+C to detach", backend)

		<-ctx.Done()
		log.Printf("detaching...")
		return nil
	})
}

func ensureNotAttached() error {
	b, err := os.ReadFile(constants.AttachPidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		_ = os.Remove(constants.AttachPidFile)
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(constants.AttachPidFile)
		return nil
	}
	if err := proc.Signal(os.Signal(nil)); err != nil {
		_ = os.Remove(constants.AttachPidFile)
		return nil
	}
	return fmt.Errorf("already attached (pid %d)", pid)
}

func writeAttachPID() error {
	return os.WriteFile(constants.AttachPidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func cleanupAttach() {
	_ = os.Remove(constants.AttachPidFile)
	_ = os.Remove(constants.AttachSocket)
}

// IsRunning reports whether an attach process is alive.
func IsRunning() bool {
	b, err := os.ReadFile(constants.AttachPidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(os.Signal(nil)) == nil
}
