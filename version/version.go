package version

var (
	CurrentCommit string

	Version = "v2.10.0-rc5"
)

func UserVersion() string {
	return Version + CurrentCommit
}
