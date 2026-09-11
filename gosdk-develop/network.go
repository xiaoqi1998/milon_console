package milon

type Network struct {
	Name    string
	ChainId uint64
	RpcUrl  string
	InxUrl  string
}

var LocalNet = Network{
	Name:    "localNet",
	ChainId: 900_000_001,
	RpcUrl:  "http://127.0.0.1:6280/v1/rpc",
}

var DevNet = Network{
	Name:    "devNet",
	ChainId: 900_000_001,
	RpcUrl:  "https://devnet.milonlabs.com/v1/rpc",
}
