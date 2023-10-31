package version

var (
	CurrentCommit string

	Version = "v2.10.0-rc4"
)

func UserVersion() string {
	return Version + CurrentCommit
}
