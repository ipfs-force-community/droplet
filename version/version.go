package version

var (
	CurrentCommit string

	Version = "v2.5.1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
