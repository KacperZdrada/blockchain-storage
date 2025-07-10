package network

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
)

func StartNode(port int, bootstrapAddr string, protocol string) error {
	// Context created for many of the network calls
	ctx := context.Background()

	// Generate a key pair for the node's identity
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		return err
	}

	// Create a libp2p node
	host, err := libp2p.New(libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)), libp2p.Identity(priv))
	if err != nil {
		return err
	}

	// TODO: Add in protocol handlers

	// Create a local distributed hash table for peer discovery
	// Its mode is set to server so that it can respond to query requests
	// As every node is on a private network, all nodes should act as servers
	localDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))

	// TODO: Allow multiple bootstrap peers to be added
	var bootstrapPeers []*peer.AddrInfo

	if bootstrapAddr != "" {
		// Convert the address string into an address object
		addr, err := multiaddr.NewMultiaddr(bootstrapAddr)
		if err != nil {
			return err
		}

		// Get peer ID and address
		peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return err
		}

		// Add the peer info to list of bootstrap peers
		bootstrapPeers = append(bootstrapPeers, peerInfo)
	}

	// TODO: connectToBootstrapPeers()

	// Create a helper discovery object with the local DHT as its routing system
	// It acts as a high-level API for discovery operations with the DHT
	routingDiscovery := routing.NewRoutingDiscovery(localDHT)

	// Advertise that the newly created node is accepting requests on the provided protocol
	util.Advertise(ctx, routingDiscovery, protocol)

	// Temporarily block forever with a select statement (will be removed)
	select {}
}
