package version

var (
	CurrentCommit string

	Version = "v2.14.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
