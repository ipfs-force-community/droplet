package version

var (
	CurrentCommit string

	Version = "v2.2.0-rc1"
)

func UserVersion() string {
	return "venus-market " + Version + " " + CurrentCommit
}
