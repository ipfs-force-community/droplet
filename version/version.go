package version

var (
	CurrentCommit string

	Version = "v2.3.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
