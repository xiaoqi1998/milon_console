package milon

type NetworkConfig struct {
	Name    string
	ChainId uint64
	RpcUrl  string
	InxUrl  string // 预留：INX（索引节点）URL，当前未使用
}

var LocalNetConfig = NetworkConfig{
	Name:    "localNet",
	ChainId: 900_000_000, // 与 DevNet 不同，防止跨链重放
	RpcUrl:  "http://127.0.0.1:6280/milon/v1",
}

// DevNetConfig DevNet 网络配置。
// 警告：DevNet 使用 HTTP 明文 + 公网 IP，仅用于早期开发测试，不可用于生产环境。
// 生产环境请使用自定义 NetworkConfig 并配置 HTTPS RPC URL。
var DevNetConfig = NetworkConfig{
	Name:    "devNet",
	ChainId: 900_000_001,
	RpcUrl:  "http://47.84.39.153:6280/milon/v1",
}

var NamedNetworks map[string]NetworkConfig

func init() {
	NamedNetworks = make(map[string]NetworkConfig, 4)

	NamedNetworks[LocalNetConfig.Name] = LocalNetConfig
	NamedNetworks[DevNetConfig.Name] = DevNetConfig
}
