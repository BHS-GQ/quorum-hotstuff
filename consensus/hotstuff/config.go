package hotstuff

type SelectProposerPolicy uint64

const (
	RoundRobin SelectProposerPolicy = iota
	Sticky
	VRF
)

type Config struct {
	RequestTimeout uint64               `toml:",omitempty"` // The timeout for each Istanbul round in milliseconds.
	BlockPeriod    uint64               `toml:",omitempty"` // Default minimum difference between two consecutive block's timestamps in second for basic hotstuff and mill-seconds for event-driven
	LeaderPolicy   SelectProposerPolicy `toml:",omitempty"` // The policy for speaker selection
	Test           bool                 `toml:",omitempty"` // Flag for unit tests
	Epoch          uint64               `toml:",omitempty"` // The number of blocks after which to checkpoint and reset the pending votes
}

// [TODO] Modify RequestTimeout; recommit time should be > blockPeriod
var DefaultBasicConfig = &Config{
	RequestTimeout: 4000,
	BlockPeriod:    1,
	LeaderPolicy:   RoundRobin,
	Epoch:          30000,
	Test:           false,
}

var DefaultEventDrivenConfig = &Config{
	RequestTimeout: 4000,
	BlockPeriod:    2000,
	LeaderPolicy:   RoundRobin,
	Epoch:          0,
	Test:           false,
}
