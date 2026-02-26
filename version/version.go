package version

var (
	CurrentCommit string

	Version = "v2.15.1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
