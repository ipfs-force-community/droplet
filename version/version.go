package version

var (
	CurrentCommit string

	Version = "v2.7.2"
)

func UserVersion() string {
	return Version + CurrentCommit
}
