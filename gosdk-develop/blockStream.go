package milon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// Passthrough codec: bypasses protobuf and sends/receives raw Postcard bytes directly
type rawFrame struct{ payload []byte }
type rawCodec struct{}

func (rawCodec) Marshal(v any) ([]byte, error)   { return v.(*rawFrame).payload, nil }
func (rawCodec) Unmarshal(d []byte, v any) error { v.(*rawFrame).payload = d; return nil }
func (rawCodec) Name() string                    { return "postcard" }

// Stream frame kinds received (v2 protocol: ServerCommand has only three variants: Block/CaughtUp/Pong)
const (
	FrameKindBlock    uint64 = 0 // Block
	FrameKindCaughtUp uint64 = 1 // CaughtUp
	FrameKindPong     uint64 = 2 // Pong
)

// ErrTxDecode marks "a transaction within a block frame failed postcard decoding", distinguishing it from network/transport errors.
// A decode failure almost always means the client decoder is out of sync with the on-chain protocol (e.g. a field missed during a protocol upgrade); it is a systemic anomaly rather than a transient fault.
// Consumers can check errors.Is(err, ErrTxDecode) and then decide for themselves whether to halt and alert or reconnect and retry,
// instead of blindly reconnecting as if it were an ordinary network error. The error returned by Next/StreamBlocks wraps this sentinel.
var ErrTxDecode = errors.New("milon: tx history decode failed")

func putVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// encodeSubscribe Subscribe(v2): variant 0, field order consumer_id / start_height / max_in_flight
func encodeSubscribe(consumerID string, startHeight uint64, maxInFlight uint32) []byte {
	b := putVarint(nil, 0)                    // variant index, written only once
	b = putVarint(b, uint64(len(consumerID))) // ① consumerID
	b = append(b, consumerID...)
	b = putVarint(b, startHeight)            // ② startHeight
	return putVarint(b, uint64(maxInFlight)) // ③ maxInFlight
}

// encodeAck Ack: variant 1
func encodeAck(height uint64) []byte {
	return putVarint([]byte{1}, height)
}

// StreamFrame is a frame received from the stream
type StreamFrame struct {
	Kind           uint64
	BlockHeight    uint64 // valid when Kind=FrameKindBlock
	Histories      []*FramedHistory
	BlockRawLen    int
	CaughtUpHeight uint64 // valid when Kind=FrameKindCaughtUp
}

// BlockStream is a connection to the chain node's gRPC bidirectional stream (low-level control interface; can be used with your own ctx)
type BlockStream struct {
	conn   *grpc.ClientConn
	stream grpc.ClientStream
}

// streamMaxRecvMsgSize is the per-frame receive limit: gRPC defaults to 4MB and exceeding it reports ResourceExhausted;
// after the stream breaks and reconnects the server resends the same block and fails again, getting stuck on that block forever; raise it to 32MB to accommodate oversized blocks.
const streamMaxRecvMsgSize = 32 << 20

// NewBlockStream connects to the node's gRPC bidirectional stream
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

// Subscribe sends a block subscription request (v2 no longer has the includeTx parameter)
func (s *BlockStream) Subscribe(consumerID string, startHeight uint64, maxInFlight uint32) error {
	return s.stream.SendMsg(&rawFrame{payload: encodeSubscribe(consumerID, startHeight, maxInFlight)})
}

// Ack acknowledges block consumption
func (s *BlockStream) Ack(height uint64) error {
	return s.stream.SendMsg(&rawFrame{payload: encodeAck(height)})
}

// Next receives and parses the next frame
func (s *BlockStream) Next() (*StreamFrame, error) {
	var frame rawFrame
	if err := s.stream.RecvMsg(&frame); err != nil {
		return nil, err
	}
	return decodeStreamFrame(frame.payload)
}

// Close closes the underlying connection
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
		//V2 BlockEnvelopeHeader(block_envelope.rs): u16/u64/u32 are all varint, hash is fixed-length;
		//field order: envelopeVersion/chainID/height/epoch/slot/blockHash/parentHash/stateHash/txRoot/txCount/timestampMsecs/schemaSetID/payloadChecksum
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
		if _, err = r.varint(); err != nil { //slot (newly added field, envelope_version unchanged; failing to read it would misalign all subsequent fields)
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
			txHistory, err := r.bytesVar() //postcard bytes of FramedHistory
			if err != nil {
				return nil, err
			}
			fh, err := decodeFramedHistory(txHistory)
			if err != nil {
				// A single decode failure = the decoder is out of sync with the on-chain protocol (e.g. a protocol upgrade changed the postcard fields); it is a systemic anomaly:
				// silently skipping is not allowed—the block would still advance and all resource changes of that transaction (balances/holders/accounts/SFT) would be lost without a trace.
				// Wrap it with the ErrTxDecode sentinel so consumers can check errors.Is and halt with an alert; un-Acked blocks can be rescanned after reconnection.
				return nil, fmt.Errorf("%w: tx[%d] at block %d: %v", ErrTxDecode, i, frame.BlockHeight, err)
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
// Client.StreamBlocks — high-level block stream entry point
// ========================================

// streamMaxInFlight is the default subscription parameter max_in_flight for StreamBlocks: the in-flight limit of un-Acked blocks
const streamMaxInFlight = 64

// StreamBlocks subscribes to the chain node's block stream starting from startBlock, invoking onBlockDone for each block.
//
// Two parameters:
//  1. startBlock  —— the starting block height of the subscription (e.g. 0 means starting from the genesis block);
//  2. onBlockDone —— called when all transactions of the current block have been fetched, receiving the block height and the decoded transaction list;
//     return true to continue fetching the next block (the SDK automatically Acks the current block), or false to stop fetching and return nil normally.
func (client *Client) StreamBlocks(startBlock uint64, onBlockDone func(blockHeight uint64, histories []*FramedHistory) bool) error {
	if client.network.GrpcAddr == "" {
		return fmt.Errorf("network %q has no grpc address configured", client.network.Name)
	}

	stream, err := NewBlockStream(context.Background(), client.network.GrpcAddr)
	if err != nil {
		return err
	}
	defer stream.Close()

	// consumer_id must be unique (a duplicate ID is rejected when the server enables uniqueness validation); use an in-process nanosecond timestamp to ensure uniqueness;
	// the character set is limited to [a-zA-Z0-9._-:] and the length to 1..=128
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
				return nil // callback requested to stop fetching
			}
			if err = stream.Ack(frame.BlockHeight); err != nil {
				return err
			}
		case FrameKindCaughtUp, FrameKindPong:
			// chain-tip notifications and heartbeat frames do not trigger the callback; keep waiting
		default:
			// unknown frame kind (new variant added by protocol evolution): ignore
		}
	}
}
