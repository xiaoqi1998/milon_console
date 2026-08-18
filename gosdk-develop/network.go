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
	RpcUrl:  "http://127.0.0.1:6280/milon/v1",
}

var DevNet = Network{
	Name:    "devNet",
	ChainId: 900_000_001,
	RpcUrl:  "http://47.84.39.153:6280/milon/v1",
}
