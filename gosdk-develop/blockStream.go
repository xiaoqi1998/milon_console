package milon

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// 透传 codec: 绕过 protobuf，直接收发 Postcard 原始字节
type rawFrame struct{ payload []byte }
type rawCodec struct{}

func (rawCodec) Marshal(v any) ([]byte, error)   { return v.(*rawFrame).payload, nil }
func (rawCodec) Unmarshal(d []byte, v any) error { v.(*rawFrame).payload = d; return nil }
func (rawCodec) Name() string                    { return "postcard" }

// 收到的流帧类型(v2 协议: ServerCommand 只有 Block/CaughtUp/Pong 三个变体)
const (
	FrameKindBlock    uint64 = 0 // Block
	FrameKindCaughtUp uint64 = 1 // CaughtUp
	FrameKindPong     uint64 = 2 // Pong
)

func putVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// encodeSubscribe Subscribe(v2): variant 0,字段顺序 consumer_id / start_height / max_in_flight
func encodeSubscribe(consumerID string, startHeight uint64, maxInFlight uint32) []byte {
	b := putVarint(nil, 0)                    // variant 索引,只写一次
	b = putVarint(b, uint64(len(consumerID))) // ① consumerID
	b = append(b, consumerID...)
	b = putVarint(b, startHeight)            // ② startHeight
	return putVarint(b, uint64(maxInFlight)) // ③ maxInFlight
}

// encodeAck Ack: variant 1
func encodeAck(height uint64) []byte {
	return putVarint([]byte{1}, height)
}

// StreamFrame 从流中收到的一帧
type StreamFrame struct {
	Kind           uint64
	BlockHeight    uint64 // Kind=FrameKindBlock 时有效
	Histories      []*FramedHistory
	BlockRawLen    int
	CaughtUpHeight uint64 // Kind=FrameKindCaughtUp 时有效
}

// BlockStream 指向链节点 gRPC 双向流的连接(底层控制接口,可自行搭配 ctx 使用)
type BlockStream struct {
	conn   *grpc.ClientConn
	stream grpc.ClientStream
}

// streamMaxRecvMsgSize 单帧接收上限: gRPC 默认 4MB,超出会报 ResourceExhausted,
// 断流重连后服务端重发同一块又失败,永远卡在该块;放大到 32MB 兜住超大块。
const streamMaxRecvMsgSize = 32 << 20

// NewBlockStream 连接节点的 gRPC 双向流
func NewBlockStream(ctx context.Context, addr string) (*BlockStream, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                20 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(rawCodec{}), grpc.MaxCallRecvMsgSize(streamMaxRecvMsgSize)),
	)
	if err != nil {
		return nil, err
	}

	streamCtx := metadata.AppendToOutgoingContext(ctx,
		"x-milon-codec", "postcard",
		"x-milon-protocol-version", "2",
	)
	stream, err := conn.NewStream(streamCtx, &grpc.StreamDesc{
		StreamName:    "Bidi",
		ServerStreams: true,
		ClientStreams: true,
	}, "/milon.grpc.v1.Stream/Bidi")
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &BlockStream{conn: conn, stream: stream}, nil
}

// Subscribe 发送订阅区块请求(v2 已无 includeTx 参数)
func (s *BlockStream) Subscribe(consumerID string, startHeight uint64, maxInFlight uint32) error {
	return s.stream.SendMsg(&rawFrame{payload: encodeSubscribe(consumerID, startHeight, maxInFlight)})
}

// Ack 区块消费确认
func (s *BlockStream) Ack(height uint64) error {
	return s.stream.SendMsg(&rawFrame{payload: encodeAck(height)})
}

// Next 接收并解析下一帧
func (s *BlockStream) Next() (*StreamFrame, error) {
	var frame rawFrame
	if err := s.stream.RecvMsg(&frame); err != nil {
		return nil, err
	}
	return decodeStreamFrame(frame.payload)
}

// Close 关闭底层连接
func (s *BlockStream) Close() error {
	return s.conn.Close()
}

func decodeStreamFrame(payload []byte) (*StreamFrame, error) {
	r := newPcReader(payload)

	kind, err := r.varint()
	if err != nil {
		return nil, fmt.Errorf("read frame kind failed: %w", err)
	}

	frame := &StreamFrame{Kind: kind}
	switch kind {
	case FrameKindBlock:
		//V2 BlockEnvelopeHeader(block_envelope.rs): u16/u64/u32 都是 varint,hash 定长;
		//字段顺序: envelopeVersion/chainID/height/epoch/slot/blockHash/parentHash/stateHash/txRoot/txCount/timestampMsecs/schemaSetID/payloadChecksum
		if _, err = r.varint(); err != nil { //envelopeVersion u16(=1)
			return nil, err
		}
		if _, err = r.varint(); err != nil { //chainID
			return nil, err
		}
		if frame.BlockHeight, err = r.varint(); err != nil { //height
			return nil, err
		}
		if _, err = r.varint(); err != nil { //epoch
			return nil, err
		}
		if _, err = r.varint(); err != nil { //slot(新增字段,envelope_version 未变;漏读会导致后续字段全部错位)
			return nil, err
		}
		if _, err = r.fixed(32); err != nil { //blockHash
			return nil, err
		}
		if _, err = r.fixed(32); err != nil { //parentHash
			return nil, err
		}
		if _, err = r.fixed(32); err != nil { //stateHash
			return nil, err
		}
		if _, err = r.fixed(32); err != nil { //txRoot
			return nil, err
		}
		if _, err = r.varint(); err != nil { //txCount u32
			return nil, err
		}
		if _, err = r.varint(); err != nil { //timestampMsecs
			return nil, err
		}
		if _, err = r.varint(); err != nil { //schemaSetID
			return nil, err
		}
		if _, err = r.fixed(32); err != nil { //payloadChecksum
			return nil, err
		}

		blockRaw, err := r.bytesVar()
		if err != nil {
			return nil, err
		}
		frame.BlockRawLen = len(blockRaw)

		txCount, err := r.varint()
		if err != nil {
			return nil, err
		}
		frame.Histories = make([]*FramedHistory, 0, txCount)
		for i := 0; i < int(txCount); i++ {
			txHistory, err := r.bytesVar() //FramedHistory 的 postcard 字节
			if err != nil {
				return nil, err
			}
			fh, err := decodeFramedHistory(txHistory)
			if err != nil {
				// 单笔解码失败 = 解码器与链上协议错位(如协议升级导致 postcard 字段变化),属于系统性异常:
				// 不允许静默跳过——本块会照常推进,该交易的全部资源变更(余额/holders/账户/SFT)将无迹可寻。
				// 返回 error 让调用方中断本块(未 Ack 的块重连后可重扫)。
				return nil, fmt.Errorf("decode tx[%d] failed at block %d: %w", i, frame.BlockHeight, err)
			}
			frame.Histories = append(frame.Histories, fh)
		}
	case FrameKindCaughtUp:
		frame.CaughtUpHeight, err = r.varint()
		if err != nil {
			return nil, err
		}
	case FrameKindPong:
	}

	return frame, nil
}

// ========================================
// Client.StreamBlocks — 区块流高层入口
// ========================================

// streamMaxInFlight StreamBlocks 默认订阅参数 max_in_flight: 未 Ack 块的在途上限
const streamMaxInFlight = 64

// StreamBlocks 从 startBlock 开始订阅链节点的区块流,逐块回调 onBlockDone。
//
// 两个入参:
//  1. startBlock  —— 订阅起始区块高度(如 0 表示从创世块开始);
//  2. onBlockDone —— 当前区块的交易全部获取完毕时被调用,入参为本块高度与已解码的交易列表;
//     返回 true 继续获取下一块(SDK 自动 Ack 当前块),返回 false 停止获取并正常返回 nil。
func (client *Client) StreamBlocks(startBlock uint64, onBlockDone func(blockHeight uint64, histories []*FramedHistory) bool) error {
	if client.network.GrpcAddr == "" {
		return fmt.Errorf("network %q has no grpc address configured", client.network.Name)
	}

	stream, err := NewBlockStream(context.Background(), client.network.GrpcAddr)
	if err != nil {
		return err
	}
	defer stream.Close()

	// consumer_id 需唯一(服务端开启唯一性校验时重复 ID 会被拒绝),用进程内纳秒时间戳保证唯一;
	// 字符集限 [a-zA-Z0-9._-:],长度 1..=128
	consumerID := fmt.Sprintf("milon-go-sdk-%d", time.Now().UnixNano())
	if err = stream.Subscribe(consumerID, startBlock, streamMaxInFlight); err != nil {
		return err
	}

	for {
		frame, err := stream.Next()
		if err != nil {
			return err
		}
		switch frame.Kind {
		case FrameKindBlock:
			if !onBlockDone(frame.BlockHeight, frame.Histories) {
				return nil // 回调要求停止获取
			}
			if err = stream.Ack(frame.BlockHeight); err != nil {
				return err
			}
		case FrameKindCaughtUp, FrameKindPong:
			// 链尖通知与心跳帧不触发回调,继续等待
		default:
			// 未知帧类型(协议演进新增变体):忽略
		}
	}
}
