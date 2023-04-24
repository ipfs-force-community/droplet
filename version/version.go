package version

var (
	CurrentCommit string

	Version = "v2.7.1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
