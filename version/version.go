package version

var (
	CurrentCommit string

	Version = "v2.13.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
