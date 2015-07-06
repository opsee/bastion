package messaging

// TODO: Migrate shared code and constantize magic strings/numbers here.

const (
	nsqdURL = "http://nsqd:4150"
)

func getNsqdURL() string {
	return nsqdURL
}
