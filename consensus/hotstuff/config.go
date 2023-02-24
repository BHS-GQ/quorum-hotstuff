package hotstuff

type SelectProposerPolicy uint64

const (
	RoundRobin SelectProposerPolicy = iota
	Sticky
	VRF
)

type FaultyMode uint64

const (
	// Disabled disables the faulty mode
	Disabled FaultyMode = iota
	// Leader sends faulty PreCommit message to <=F replicas
	TargetedWrongPreCommit
	// Leader sends faulty PreCommit message to <=F replicas
	TargetedWrongCommit
)

func (f FaultyMode) Uint64() uint64 {
	return uint64(f)
}

func (f FaultyMode) String() string {
	switch f {
	case Disabled:
		return "Disabled"
	case TargetedWrongPreCommit:
		return "TargetedWrongPreCommit"
	case TargetedWrongCommit:
		return "TargetedWrongCommit"
	default:
		return "Undefined"
	}
}

type Config struct {
	RequestTimeout uint64               `toml:",omitempty"` // The timeout for each Istanbul round in milliseconds.
	BlockPeriod    uint64               `toml:",omitempty"` // Default minimum difference between two consecutive block's timestamps in second for basic hotstuff and mill-seconds for event-driven
	LeaderPolicy   SelectProposerPolicy `toml:",omitempty"` // The policy for speaker selection
	Test           bool                 `toml:",omitempty"` // Flag for unit tests
	FaultyMode     uint64               `toml:",omitempty"` // The faulty node indicates the faulty node's behavior
	Epoch          uint64               `toml:",omitempty"` // The number of blocks after which to checkpoint and reset the pending votes
}

// [TODO] Modify RequestTimeout; recommit time should be > blockPeriod
var DefaultBasicConfig = &Config{
	RequestTimeout: 6000,
	BlockPeriod:    3,
	LeaderPolicy:   RoundRobin,
	Epoch:          30000,
	Test:           false,
	FaultyMode:     TargetedWrongCommit.Uint64(),
}
