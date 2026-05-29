package trace

type Backend string

const (
	BackendVfkit Backend = "vfkit"
	BackendQemu  Backend = "qemu"
	BackendBess  Backend = "bess"
)

func BackendValues() []string {
	return []string{
		string(BackendVfkit),
		string(BackendQemu),
		string(BackendBess),
	}
}
