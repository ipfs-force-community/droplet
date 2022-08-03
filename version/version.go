package version

var (
	CurrentCommit string

	Version = "v2.2.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
