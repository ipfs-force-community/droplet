package version

var (
	CurrentCommit string

	Version = "v2.12.0-rc1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
