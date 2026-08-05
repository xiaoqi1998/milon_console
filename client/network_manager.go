package client

import (
	"fmt"
	"sync"

	milon "github.com/milon-labs/milon-go-sdk"
)

// NetworkInfo is the DTO returned by the network management API.
type NetworkInfo struct {
	Name    string `json:"name"`
	ChainId uint64 `json:"chainId"`
	RpcUrl  string `json:"rpcUrl"`
	InxUrl  string `json:"inxUrl"`
	Current bool   `json:"current"`
}

// NetworkManager manages multiple Milon network configurations and their clients.
type NetworkManager struct {
	mu             sync.RWMutex
	networks       map[string]milon.Network
	currentNetwork string
	clients        map[string]*milon.Client
}

// NewNetworkManager creates a NetworkManager with localNet and devNet pre-configured,
// then creates a client for the default network.
func NewNetworkManager(defaultNetwork string) *NetworkManager {
	nm := &NetworkManager{
		networks: make(map[string]milon.Network),
		clients:  make(map[string]*milon.Client),
	}

	nm.networks[milon.LocalNet.Name] = milon.LocalNet
	nm.networks[milon.DevNet.Name] = milon.DevNet

	if _, ok := nm.networks[defaultNetwork]; !ok {
		defaultNetwork = milon.DevNet.Name
	}
	nm.currentNetwork = defaultNetwork

	if _, err := nm.createClient(defaultNetwork); err != nil {
		panic(fmt.Sprintf("failed to create client for default network %s: %v", defaultNetwork, err))
	}

	return nm
}

// ListNetworks returns all available networks, marking the current one.
func (nm *NetworkManager) ListNetworks() []NetworkInfo {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	list := make([]NetworkInfo, 0, len(nm.networks))
	for _, cfg := range nm.networks {
		list = append(list, NetworkInfo{
			Name:    cfg.Name,
			ChainId: cfg.ChainId,
			RpcUrl:  cfg.RpcUrl,
			InxUrl:  cfg.InxUrl,
			Current: cfg.Name == nm.currentNetwork,
		})
	}
	return list
}

// GetCurrent returns the client and config of the current network.
func (nm *NetworkManager) GetCurrent() (*milon.Client, milon.Network) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	cfg := nm.networks[nm.currentNetwork]
	client := nm.clients[nm.currentNetwork]
	return client, cfg
}

// Switch changes the current network, creating a client on demand.
func (nm *NetworkManager) Switch(networkName string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if _, ok := nm.networks[networkName]; !ok {
		return fmt.Errorf("network %s not found", networkName)
	}

	if _, ok := nm.clients[networkName]; !ok {
		if _, err := nm.createClient(networkName); err != nil {
			return err
		}
	}

	nm.currentNetwork = networkName
	return nil
}

// getOrCreateClient returns a cached client or creates one on demand.
func (nm *NetworkManager) getOrCreateClient(networkName string) (*milon.Client, error) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if client, ok := nm.clients[networkName]; ok {
		return client, nil
	}

	return nm.createClient(networkName)
}

// createClient builds a MilonClient for the given network. Caller must hold the lock.
// Note: NewClient does not return an error (IDL load failure panics), so we recover
// any panic and convert it into a readable error to keep the existing error flow.
func (nm *NetworkManager) createClient(networkName string) (client *milon.Client, err error) {
	cfg, ok := nm.networks[networkName]
	if !ok {
		return nil, fmt.Errorf("network %s not found", networkName)
	}

	defer func() {
		if r := recover(); r != nil {
			client = nil
			err = fmt.Errorf("failed to create MilonClient for network %s: %v", networkName, r)
		}
	}()

	client = milon.NewClient(cfg)

	nm.clients[networkName] = client
	return client, nil
}