package types

import (
	"errors"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	// HotstuffDigest represents a hash of "Hotstuff practical byzantine fault tolerance"
	// to identify whether the block is from Hotstuff consensus engine
	HotstuffDigest = common.HexToHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	HotstuffExtraVanity = 32 // Fixed number of extra-data bytes reserved for validator vanity
	HotstuffExtraSeal   = 65 // Fixed number of extra-data bytes reserved for validator seal

	// ErrInvalidHotstuffHeaderExtra is returned if the length of extra-data is less than 32 bytes
	ErrInvalidHotstuffHeaderExtra = errors.New("invalid istanbul header extra-data")
)

type HotstuffExtra struct {
	Validators    []common.Address
	Seal          []byte
	CommittedSeal [][]byte
	Salt          []byte
}

// EncodeRLP serializes ist into the Ethereum RLP format.
func (ist *HotstuffExtra) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		ist.Validators,
		ist.Seal,
		ist.CommittedSeal,
		ist.Salt,
	})
}

// DecodeRLP implements rlp.Decoder, and load the istanbul fields from a RLP stream.
func (ist *HotstuffExtra) DecodeRLP(s *rlp.Stream) error {
	var extra struct {
		Validators    []common.Address
		Seal          []byte
		CommittedSeal [][]byte
		Salt          []byte
	}
	if err := s.Decode(&extra); err != nil {
		return err
	}
	ist.Validators, ist.Seal, ist.CommittedSeal, ist.Salt = extra.Validators, extra.Seal, extra.CommittedSeal, extra.Salt
	return nil
}

// ExtractHotstuffExtra extracts all values of the HotstuffExtra from the header. It returns an
// error if the length of the given extra-data is less than 32 bytes or the extra-data can not
// be decoded.
func ExtractHotstuffExtra(h *Header) (*HotstuffExtra, error) {
	return ExtractHotstuffExtraPayload(h.Extra)
}

func ExtractHotstuffExtraPayload(extra []byte) (*HotstuffExtra, error) {
	if len(extra) < HotstuffExtraVanity {
		return nil, ErrInvalidHotstuffHeaderExtra
	}

	var hotstuffExtra *HotstuffExtra
	err := rlp.DecodeBytes(extra[HotstuffExtraVanity:], &hotstuffExtra)
	if err != nil {
		return nil, err
	}
	return hotstuffExtra, nil
}

// HotstuffFilteredHeader returns a filtered header which some information (like seal, committed seals)
// are clean to fulfill the Istanbul hash rules. It returns nil if the extra-data cannot be
// decoded/encoded by rlp.
func HotstuffFilteredHeader(h *Header, keepSeal bool) *Header {
	newHeader := CopyHeader(h)
	extra, err := ExtractHotstuffExtra(newHeader)
	if err != nil {
		return nil
	}

	if !keepSeal {
		extra.Seal = []byte{}
	}
	extra.CommittedSeal = [][]byte{}
	extra.Salt = []byte{}

	payload, err := rlp.EncodeToBytes(&extra)
	if err != nil {
		return nil
	}

	newHeader.Extra = append(newHeader.Extra[:HotstuffExtraVanity], payload...)

	return newHeader
}
