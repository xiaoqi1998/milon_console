package milon

type Network struct {
	Name     string
	ChainId  uint64
	RpcUrl   string
	InxUrl   string
	GrpcAddr string // block stream gRPC address, host:port, used by StreamBlocks / NewBlockStream
}

var LocalNet = Network{
	Name:     "localNet",
	ChainId:  900_000_001,
	RpcUrl:   "http://127.0.0.1:6280/v1/rpc",
	GrpcAddr: "127.0.0.1:50051",
}

var DevNet = Network{
	Name:     "devNet",
	ChainId:  900_000_001,
	RpcUrl:   "https://devnet.milonlabs.com/v1/rpc",
	GrpcAddr: "8.218.101.239:50051",
}
