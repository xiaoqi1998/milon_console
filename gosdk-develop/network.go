package milon

// Network describes a chain endpoint. ChainId and RpcUrl are required: the
// chain id is mixed into every transaction, instruction and auth hash, so the
// SDK deliberately ships no default for it.
type Network struct {
	Name     string
	ChainId  uint64 // required: mixed into tx / ix / auth hashes
	RpcUrl   string
	GrpcAddr string // block stream gRPC address, host:port, used by StreamBlocks / NewBlockStream
}

// LocalNet and DevNet currently share chain id 900_000_001: both are test
// networks of the same chain.
var LocalNet = Network{
	Name:     "localNet",
	ChainId:  900_000_001,
	RpcUrl:   "http://127.0.0.1:6280/v1/rpc",
	GrpcAddr: "127.0.0.1:50051",
}

var DevNet = Network{
	Name:     "devNet",
	ChainId:  100_10,
	RpcUrl:   "https://dev-api.milonlabs.com/v1/rpc",
	GrpcAddr: "dev-api-grpc.milonlabs.com:443",
}
