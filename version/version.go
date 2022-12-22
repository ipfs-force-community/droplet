package version

var (
	CurrentCommit string

	Version = "2.4.1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
