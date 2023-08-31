package version

var (
	CurrentCommit string

	Version = "v2.9.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
