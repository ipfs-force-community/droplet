package version

var (
	CurrentCommit string

	Version = "v2.7.3"
)

func UserVersion() string {
	return Version + CurrentCommit
}
