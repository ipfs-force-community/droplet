package version

var (
	CurrentCommit string

	Version = "v2.17.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
