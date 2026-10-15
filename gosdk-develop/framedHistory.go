package milon

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

// ---------- postcard reader ----------

type pcReader struct {
	b   []byte
	off int
}

func newPcReader(b []byte) *pcReader {
	return &pcReader{b: b}
}

func (r *pcReader) u8() (byte, error) {
	if r.off >= len(r.b) {
		return 0, fmt.Errorf("unexpected EOF at %d", r.off)
	}
	v := r.b[r.off]
	r.off++
	return v, nil
}

func (r *pcReader) varint() (uint64, error) {
	var v, shift uint64
	for {
		if r.off >= len(r.b) {
			return 0, fmt.Errorf("unexpected EOF in varint at %d", r.off)
		}
		x := r.b[r.off]
		r.off++
		v |= uint64(x&0x7f) << shift
		if x < 0x80 {
			return v, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("varint overflow")
		}
	}
}

func (r *pcReader) fixed(n int) ([]byte, error) {
	if r.off+n > len(r.b) {
		return nil, fmt.Errorf("unexpected EOF: need %d bytes at %d", n, r.off)
	}
	v := r.b[r.off : r.off+n]
	r.off += n
	return v, nil
}

func (r *pcReader) bytesVar() ([]byte, error) {
	n, err := r.varint()
	if err != nil {
		return nil, err
	}
	return r.fixed(int(n))
}

func (r *pcReader) option() (bool, error) {
	tag, err := r.u8()
	if err != nil {
		return false, err
	}
	if tag > 1 {
		return false, fmt.Errorf("bad option tag %d", tag)
	}
	return tag == 1, nil
}

// DecodeVarint reads a postcard varint from the head of b, returning the value and the number of bytes consumed
func DecodeVarint(b []byte) (value uint64, n int, err error) {
	r := newPcReader(b)
	value, err = r.varint()
	if err != nil {
		return 0, 0, err
	}
	return value, r.off, nil
}

// ---------- FramedHistory parsing (corresponds to frame_history in the server-side payload.rs) ----------

// FramedHistory is the complete persisted record of a transaction in the block stream (self-contained postcard frame).
type FramedHistory struct {
	Version      uint8
	Stamp        uint64
	Payer        *uint8 // payer is the index into the Signatures array; nil means no payer (split-pay)
	Signatures   []TxHistorySignature
	Instructions [][]byte // raw postcard bytes of each instruction
	TxID         []byte   // 12 bytes: block_id(8B BE) + index(4B BE)
	TxHash       []byte   // 32 bytes
	State        uint8
	Accesses     []FramedAccess
	Events       []FramedDynamicValue
	ErrorCode    *uint16
	GasCharged   uint64
}

// TxHistorySignature is a signer authorization record within a transaction.
type TxHistorySignature struct {
	Signer  []byte // Account: fixed 20 bytes
	AuthBit uint64 // Bitmap64: transparent u64
	SigBit  uint64
}

// FramedAccess is a transaction's access record for a resource (before/after snapshots).
type FramedAccess struct {
	ResourceID    []byte // RsHash: fixed 18 bytes
	FirstSnapshot *FramedDynamicValue
	LastWritten   FramedDynamicValue
}

// FramedDynamicValue is a dynamic value with a type_tag (resource value/event); the body can only be further parsed using the IDL corresponding to the type_tag
type FramedDynamicValue struct {
	TypeTag uint64
	Body    []byte
}

// TxHashBase58 returns the transaction hash in Base58
func (fh *FramedHistory) TxHashBase58() string {
	var txHash api.TxHash
	copy(txHash[:], fh.TxHash)
	return txHash.ToBase58()
}

// PayerAddressBase58 returns the payer address: fh.Payer is the index into the Signatures array (e.g. payer=signer[0]);
// returns "" when there is no payer (split-pay). The conversion matches the signer display in PrintFramedHistory (crypto.NewAddressFromBytes → Base58).
func (fh *FramedHistory) PayerAddressBase58() (string, error) {
	if fh.Payer == nil {
		return "", nil
	}
	idx := int(*fh.Payer)
	if idx >= len(fh.Signatures) {
		return "", fmt.Errorf("payer index %d out of range, signers=%d", idx, len(fh.Signatures))
	}
	addr, err := crypto.NewAddressFromBytes(fh.Signatures[idx].Signer)
	if err != nil {
		return "", fmt.Errorf("derive payer address from signer[%d] failed: %w", idx, err)
	}
	return addr.ToBase58(), nil
}

func decodeFramedHistory(b []byte) (*FramedHistory, error) {
	r := newPcReader(b)
	magic, err := r.fixed(4)
	if err != nil {
		return nil, err
	}
	if string(magic) != "MSDK" {
		return nil, fmt.Errorf("bad magic %x", magic)
	}
	fh := &FramedHistory{}
	if fh.Version, err = r.u8(); err != nil {
		return nil, err
	}
	if fh.Stamp, err = r.varint(); err != nil {
		return nil, err
	}
	hasPayer, err := r.option()
	if err != nil {
		return nil, err
	}
	if hasPayer {
		p, err := r.u8()
		if err != nil {
			return nil, err
		}
		fh.Payer = &p
	}
	sigCount, err := r.varint()
	if err != nil {
		return nil, err
	}
	fh.Signatures = make([]TxHistorySignature, sigCount)
	for i := range fh.Signatures {
		signer, err := r.fixed(20)
		if err != nil {
			return nil, err
		}
		authBit, err := r.varint()
		if err != nil {
			return nil, err
		}
		sigBit, err := r.varint()
		if err != nil {
			return nil, err
		}
		fh.Signatures[i] = TxHistorySignature{Signer: signer, AuthBit: authBit, SigBit: sigBit}
	}
	ixCount, err := r.varint()
	if err != nil {
		return nil, err
	}
	fh.Instructions = make([][]byte, ixCount)
	for i := range fh.Instructions {
		fh.Instructions[i], err = r.bytesVar()
		if err != nil {
			return nil, err
		}
	}
	if fh.TxID, err = r.fixed(12); err != nil {
		return nil, err
	}
	if fh.TxHash, err = r.fixed(32); err != nil {
		return nil, err
	}
	if fh.State, err = r.u8(); err != nil {
		return nil, err
	}
	accessCount, err := r.varint()
	if err != nil {
		return nil, err
	}
	fh.Accesses = make([]FramedAccess, accessCount)
	for i := range fh.Accesses {
		resourceHash, err := r.fixed(18)
		if err != nil {
			return nil, err
		}
		//V2 new field: resource_id: Option<ResourceId>; when Some, skip the full structure
		hasResID, err := r.option()
		if err != nil {
			return nil, err
		}
		if hasResID {
			if _, err = r.fixed(2); err != nil { //res_id
				return nil, err
			}
			segCount, err := r.varint() //path: Vec<PathSegment>
			if err != nil {
				return nil, err
			}
			for j := 0; j < int(segCount); j++ {
				segVariant, err := r.varint()
				if err != nil {
					return nil, err
				}
				switch segVariant {
				case 0: //Address fixed 20B
					if _, err = r.fixed(20); err != nil {
						return nil, err
					}
				case 1: //U8
					if _, err = r.u8(); err != nil {
						return nil, err
					}
				case 2, 3, 4: //U16/U32/U64
					if _, err = r.varint(); err != nil {
						return nil, err
					}
				case 5: //Bytes32
					if _, err = r.fixed(32); err != nil {
						return nil, err
					}
				default:
					return nil, fmt.Errorf("unexpected PathSegment variant %d", segVariant)
				}
			}
			if _, err = r.fixed(18); err != nil { //hash
				return nil, err
			}
		}

		hasFirst, err := r.option()
		if err != nil {
			return nil, err
		}
		fa := FramedAccess{ResourceID: resourceHash}
		if hasFirst {
			v, err := readPersistedValue(r)
			if err != nil {
				return nil, err
			}
			fa.FirstSnapshot = &v
		}
		fa.LastWritten, err = readPersistedValue(r)
		if err != nil {
			return nil, err
		}
		fh.Accesses[i] = fa
	}
	eventCount, err := r.varint()
	if err != nil {
		return nil, err
	}
	fh.Events = make([]FramedDynamicValue, eventCount)
	for i := range fh.Events {
		fh.Events[i], err = readDynamicValue(r)
		if err != nil {
			return nil, err
		}
	}
	hasErr, err := r.option()
	if err != nil {
		return nil, err
	}
	if hasErr {
		code, err := r.varint()
		if err != nil {
			return nil, err
		}
		c := uint16(code)
		fh.ErrorCode = &c
	}
	fh.GasCharged, err = r.varint()
	if err != nil {
		return nil, err
	}
	return fh, nil
}

// readPersistedValue currently has only the Inline(0) variant
func readPersistedValue(r *pcReader) (FramedDynamicValue, error) {
	variant, err := r.varint()
	if err != nil {
		return FramedDynamicValue{}, err
	}
	if variant != 0 {
		return FramedDynamicValue{}, fmt.Errorf("unexpected PersistedValue variant %d", variant)
	}
	return readDynamicValue(r)
}

func readDynamicValue(r *pcReader) (FramedDynamicValue, error) {
	tag, err := r.varint()
	if err != nil {
		return FramedDynamicValue{}, err
	}
	body, err := r.bytesVar()
	if err != nil {
		return FramedDynamicValue{}, err
	}
	return FramedDynamicValue{TypeTag: tag, Body: body}, nil
}
