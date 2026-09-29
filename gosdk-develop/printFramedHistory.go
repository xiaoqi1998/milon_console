package milon

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// PrintFramedHistory 打印一笔交易的完整明细:
// 基本信息 + 签名者 + 指令 IDL 解码 + 资源访问 before/after + 事件解码
func PrintFramedHistory(client *Client, idx int, fh *FramedHistory, pathMap map[api.RsHash]string) {
	if client == nil {
		return
	}
	pm := client.GetProviderManager()

	var sb strings.Builder
	blockID := binary.BigEndian.Uint64(fh.TxID[0:8])
	indexInBlock := binary.BigEndian.Uint32(fh.TxID[8:12])
	stateName := map[uint8]string{
		api.TxStatePending: "Pending",
		api.TxStateSuccess: "Success",
		api.TxStateFailed:  "Failed",
	}[fh.State]

	fmt.Fprintf(&sb, "tx[%d] tx_hash=%s tx_id=%d:%d state=%d(%s) gas=%d\n", idx, fh.TxHashBase58(), blockID, indexInBlock, fh.State, stateName, fh.GasCharged)

	if fh.Stamp > 0 {
		fmt.Fprintf(&sb, "  stamp=%d (%s)\n", fh.Stamp, time.UnixMilli(int64(fh.Stamp)).Format(time.RFC3339))
	} else {
		fmt.Fprintf(&sb, "  stamp=%d\n", fh.Stamp)
	}
	if fh.ErrorCode != nil {
		fmt.Fprintf(&sb, "  error=%d\n", *fh.ErrorCode)
	}
	if fh.Payer != nil {
		fmt.Fprintf(&sb, "  payer=signer[%d]\n", *fh.Payer)
	} else {
		fmt.Fprintf(&sb, "  payer=<none 分账>\n")
	}
	for i, sig := range fh.Signatures {
		addrStr := hex.EncodeToString(sig.Signer)
		if addr, err := crypto.NewAddressFromBytes(sig.Signer); err == nil {
			addrStr = addr.ToBase58()
		}
		fmt.Fprintf(&sb, "  signer[%d] addr=%s auth_bit=%d sig_bit=%d\n", i, addrStr, sig.AuthBit, sig.SigBit)
	}

	for i, ix := range fh.Instructions {
		if len(ix) >= 3 {
			appID := ix[0]
			disc := binary.LittleEndian.Uint16(ix[1:3])
			fmt.Fprintf(&sb, "  ix[%d] app_id=%d discriminator=%d\n", i, appID, disc)
		}
		decoded, err := pm.DecodeInstruction(ix)
		if err != nil {
			fmt.Fprintf(&sb, "    | decode failed: %v, raw=%s\n", err, hexPreview(ix))
			continue
		}
		appendIndented(&sb, pm.FormatDecodedInstruction(decoded))
	}

	for i, ac := range fh.Accesses {
		var rs api.RsHash
		copy(rs[:], ac.ResourceID)
		path, ok := pathMap[rs]
		if !ok {
			path = "<unknown>"
		}
		fmt.Fprintf(&sb, "  access[%d] resource_id=%v path=%s\n", i, ac.ResourceID, path)
		if ac.FirstSnapshot != nil {
			writeDynamicValue(client, &sb, "before", ac.FirstSnapshot)
		} else {
			sb.WriteString("    | before: <created 新资源>\n")
		}
		writeDynamicValue(client, &sb, "after", &ac.LastWritten)
	}

	for i, ev := range fh.Events {
		fmt.Fprintf(&sb, "  event[%d] tag=%d\n", i, ev.TypeTag)
		decoded, err := pm.DecodeEventDataByTag(ev.TypeTag, ev.Body)
		if err != nil {
			fmt.Fprintf(&sb, "    | decode failed: %v, raw=%s\n", err, hexPreview(ev.Body))
			continue
		}
		appendIndented(&sb, pm.FormatDecodedEvent(decoded))
	}

	fmt.Println(strings.TrimRight(sb.String(), "\n"))
}

// appendIndented 把多行解码结果逐行加前缀写入,使其与上面的 ix[n]/event[n] 标题行视觉归组
func appendIndented(sb *strings.Builder, s string) {
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		sb.WriteString("    | " + line + "\n")
	}
}

// writeDynamicValue 按 type_tag 找到 IDL 类型并解码 Inline body(postcard 资源值字节)
func writeDynamicValue(client *Client, sb *strings.Builder, label string, v *FramedDynamicValue) {
	pd, idlType := findIDLTypeByTag(client, v.TypeTag)
	if pd == nil || idlType == nil {
		fmt.Fprintf(sb, "    | %s: unknown type_tag=%d body=%s\n", label, v.TypeTag, hexPreview(v.Body))
		return
	}
	decoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, v.Body)
	if err != nil {
		fmt.Fprintf(sb, "    | %s: decode %s failed: %v, body=%s\n", label, idlType.Name, err, hexPreview(v.Body))
		return
	}
	fmt.Fprintf(sb, "    | %s: %s = %+v\n", label, idlType.Name, decoded)
}

// findIDLTypeByTag 遍历全部已加载的 IDL,按 type_tag 找到对应类型
func findIDLTypeByTag(client *Client, typeTag uint64) (*provider.Provider, *provider.IDLType) {
	for _, pd := range client.GetAllPd() {
		if idlType, ok := pd.GetIDLTypeByTypeTag(typeTag); ok {
			return pd, idlType
		}
	}
	return nil, nil
}

func hexPreview(b []byte) string {
	return fmt.Sprintf("0x%s (%dB)", hex.EncodeToString(b), len(b))
}
