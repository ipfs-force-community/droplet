package version

var (
	CurrentCommit string

	Version = "2.4.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
