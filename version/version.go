package version

var (
	CurrentCommit string

	Version = "v2.8.2"
)

func UserVersion() string {
	return Version + CurrentCommit
}
