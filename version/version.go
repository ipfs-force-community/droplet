package version

var (
	CurrentCommit string

	Version = "v2.8.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
