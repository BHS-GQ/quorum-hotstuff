package types

import (
	"bytes"
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
	ErrInvalidHotstuffHeaderExtra = errors.New("invalid hotstuff header extra-data")
)

type HotstuffExtra struct {
	Validators []common.Address

	EncodedQC []byte

	Seal []byte
	Salt []byte
}

// EncodeRLP serializes ist into the Ethereum RLP format.
func (ist *HotstuffExtra) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		ist.Validators,
		ist.EncodedQC,
		ist.Seal,
		ist.Salt,
	})
}

// DecodeRLP implements rlp.Decoder, and load the istanbul fields from a RLP stream.
func (ist *HotstuffExtra) DecodeRLP(s *rlp.Stream) error {
	var extra struct {
		Validators []common.Address
		EncodedQC  []byte
		Seal       []byte
		Salt       []byte
	}
	if err := s.Decode(&extra); err != nil {
		return err
	}
	ist.Validators, ist.Seal, ist.EncodedQC, ist.Salt = extra.Validators, extra.Seal, extra.EncodedQC, extra.Salt
	return nil
}

func (h *Header) SetEncodedQC(encodedQC []byte) error {
	extra, err := ExtractHotstuffExtra(h)
	if err != nil {
		return err
	}
	extra.EncodedQC = encodedQC
	payload, err := rlp.EncodeToBytes(&extra)
	if err != nil {
		return err
	}
	h.Extra = append(h.Extra[:HotstuffExtraVanity], payload...)
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

func (h *Header) SetSeal(seal []byte) error {
	extra, err := ExtractHotstuffExtra(h)
	if err != nil {
		return err
	}
	extra.Seal = seal
	payload, err := rlp.EncodeToBytes(&extra)
	if err != nil {
		return err
	}
	h.Extra = append(h.Extra[:HotstuffExtraVanity], payload...)
	return nil
}

// HotstuffFilteredHeader returns a filtered header which some information (like seal, bls seal)
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
	extra.EncodedQC = []byte{}
	extra.Salt = []byte{}

	payload, err := rlp.EncodeToBytes(&extra)
	if err != nil {
		return nil
	}

	newHeader.Extra = append(newHeader.Extra[:HotstuffExtraVanity], payload...)

	return newHeader
}

func HotstuffHeaderFillWithValidators(header *Header, vals []common.Address) error {
	var buf bytes.Buffer

	// compensate the lack bytes if header.Extra is not enough IstanbulExtraVanity bytes.
	if len(header.Extra) < HotstuffExtraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, HotstuffExtraVanity-len(header.Extra))...)
	}
	buf.Write(header.Extra[:HotstuffExtraVanity])

	if vals == nil {
		vals = []common.Address{}
	}
	ist := &HotstuffExtra{
		Validators: vals,
		EncodedQC:  []byte{},
		Seal:       []byte{},
		Salt:       []byte{},
	}

	payload, err := rlp.EncodeToBytes(&ist)
	if err != nil {
		return err
	}
	header.Extra = append(buf.Bytes(), payload...)
	return nil
}

func GenerateExtraWithSignature(epochStartHeight, epochEndHeight uint64, vals []common.Address, seal []byte, encodedQC []byte) ([]byte, error) {
	var (
		buf   bytes.Buffer
		extra []byte
	)

	extra = append(extra, bytes.Repeat([]byte{0x00}, HotstuffExtraVanity-len(extra))...)
	buf.Write(extra[:HotstuffExtraVanity])

	if vals == nil {
		vals = []common.Address{}
	}
	ist := &HotstuffExtra{
		Validators: vals,
		Seal:       seal,
		EncodedQC:  encodedQC,
		Salt:       []byte{},
	}

	payload, err := rlp.EncodeToBytes(&ist)
	if err != nil {
		return nil, err
	}
	extra = append(buf.Bytes(), payload...)
	return extra, nil
}
