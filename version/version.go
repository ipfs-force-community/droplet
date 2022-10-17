package version

var (
	CurrentCommit string

	Version = "v2.4.0-rc1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
