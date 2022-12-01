package version

var (
	CurrentCommit string

	Version = "v2.5.0-pre-rc2"
)

func UserVersion() string {
	return Version + CurrentCommit
}
