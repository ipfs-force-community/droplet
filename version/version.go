package version

var (
	GitCommit string

	Version = "v2.0.0-rc3"
)

func UserVersion() string {
	return "venus-market " + Version + " " + GitCommit
}
