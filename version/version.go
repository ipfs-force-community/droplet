package version

var (
	CurrentCommit string

	Version = "v2.9.1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
