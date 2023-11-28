package version

var (
	CurrentCommit string

	Version = "v2.10.0"
)

func UserVersion() string {
	return Version + CurrentCommit
}
