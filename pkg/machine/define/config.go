package define

import "os"

const UserCertsTargetPath = "/etc/containers/certs.d"
const DefaultIdentityName = "machine"

var (
	DefaultFilePerm os.FileMode = 0644
)

type CreateVMOpts struct {
	Name string
	Dirs *MachineDirs
}

type MachineDirs struct {
	ConfigDir  string
	DataDir    string
	RuntimeDir string
}
