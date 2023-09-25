package version

var (
	CurrentCommit string

	Version = "v2.8.3"
)

func UserVersion() string {
	return Version + CurrentCommit
}
