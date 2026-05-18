package version

var (
	CurrentCommit string

	Version = "v2.16.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
