package version

var (
	CurrentCommit string

	Version = "v2.5.0-pre-rc1"
)

func UserVersion() string {
	return Version + CurrentCommit
}
