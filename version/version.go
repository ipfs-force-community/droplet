package version

var (
	CurrentCommit string

	Version = "v2.0.0"
)

func UserVersion() string {
	return "venus-market " + Version + " " + CurrentCommit
}
