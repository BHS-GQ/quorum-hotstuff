package mock

import (
	"testing"
	"time"
)

// go test -v -count=1 github.com/ethereum/go-ethereum/consensus/hotstuff/mock -run TestMockPrepareCase1
// net scale is 4, leader send fake message of prepare with wrong height, repos change view.
func TestMockPrepareCase1(t *testing.T) {
	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	sys.Close(60)
}
