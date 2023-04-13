package version

var (
	CurrentCommit string

	Version = "v2.6.1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
