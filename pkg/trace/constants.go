package trace

type Backend string

const (
	BackendVfkit  Backend = "vfkit"
	BackendQemu   Backend = "qemu"
	BackendBess   Backend = "bess"
	BackendVpnkit Backend = "vpnkit"
	BackendStdio  Backend = "stdio"
)

func BackendValues() []string {
	return []string{
		string(BackendVfkit),
		string(BackendQemu),
		string(BackendBess),
		string(BackendVpnkit),
		string(BackendStdio),
	}
}
