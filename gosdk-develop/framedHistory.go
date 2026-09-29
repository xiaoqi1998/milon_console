package milon

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

// ---------- postcard 读取器 ----------

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

// DecodeVarint 从 b 头部读一个 postcard varint，返回值与消耗的字节数
func DecodeVarint(b []byte) (value uint64, n int, err error) {
	r := newPcReader(b)
	value, err = r.varint()
	if err != nil {
		return 0, 0, err
	}
	return value, r.off, nil
}

// ---------- FramedHistory 解析(对应服务端 payload.rs 的 frame_history) ----------

// FramedHistory 区块流中一笔交易的完整持久化记录(postcard 自包含帧)。
type FramedHistory struct {
	Version      uint8
	Stamp        uint64
	Payer        *uint8 // payer 是 Signatures 数组的索引；nil 表示无 payer(分账)
	Signatures   []TxHistorySignature
	Instructions [][]byte // 每条指令的 postcard 原始字节
	TxID         []byte   // 12 字节: block_id(8B BE) + index(4B BE)
	TxHash       []byte   // 32 字节
	State        uint8
	Accesses     []FramedAccess
	Events       []FramedDynamicValue
	ErrorCode    *uint16
	GasCharged   uint64
}

// TxHistorySignature 一笔交易中的一个签名者授权记录。
type TxHistorySignature struct {
	Signer  []byte // Account: 定长 20 字节
	AuthBit uint64 // Bitmap64: 透明 u64
	SigBit  uint64
}

// FramedAccess 一笔交易对一个资源的访问记录(before/after 快照)。
type FramedAccess struct {
	ResourceID    []byte // RsHash: 定长 18 字节
	FirstSnapshot *FramedDynamicValue
	LastWritten   FramedDynamicValue
}

// FramedDynamicValue 带 type_tag 的动态值(资源值/事件)，body 需按 type_tag 对应的 IDL 才能继续解析
type FramedDynamicValue struct {
	TypeTag uint64
	Body    []byte
}

// TxHashBase58 交易哈希 Base58
func (fh *FramedHistory) TxHashBase58() string {
	var txHash api.TxHash
	copy(txHash[:], fh.TxHash)
	return txHash.ToBase58()
}

// PayerAddressBase58 付款方地址: fh.Payer 是 Signatures 数组的索引(如 payer=signer[0]);
// 无 payer(分账)时返回 ""。转换规则与 PrintFramedHistory 的 signer 展示一致(crypto.NewAddressFromBytes → Base58)。
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
		//V2 新增字段: resource_id: Option<ResourceId>, 为 Some 时跳过完整结构
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
				case 0: //Address 定长 20B
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

// readPersistedValue 目前只有 Inline(0) 一个变体
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
