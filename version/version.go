package version

var (
	CurrentCommit string

	Version = "v2.8.1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
