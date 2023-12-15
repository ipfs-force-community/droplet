package version

var (
	CurrentCommit string

	Version = "v2.10.1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
