package version

var (
	CurrentCommit string

	Version = "v2.11.1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
