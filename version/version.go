package version

var (
	CurrentCommit string

	Version = "v2.7.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
