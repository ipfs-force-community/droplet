package version

var (
	GitCommit string

	Version = "1.0.0"
)

func UserVersion() string {
	return "venus-market " + Version + " " + GitCommit
}
