package network

import (
	"context"
	"errors"
	"fmt"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
	"sync"
)

var Peers []*peer.AddrInfo
var PeersMutex = &sync.Mutex{}

func StartNode(port int, bootstrapAddr string) error {
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

	host.SetStreamHandler(protocol, handleStream)

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

	// Check if bootstrap peers provided and if so connect to them
	if len(bootstrapPeers) > 0 {
		err := connectToBootstrapPeers(ctx, host, bootstrapPeers)
		if err != nil {
			return err
		}
	}

	// Create a helper discovery object with the local DHT as its routing system
	// It acts as a high-level API for discovery operations with the DHT
	routingDiscovery := routing.NewRoutingDiscovery(localDHT)

	// Advertise that the newly created node is accepting requests on the provided protocol
	util.Advertise(ctx, routingDiscovery, protocol)

	// Attempt to discover other peers
	go discoverPeers(ctx, host, routingDiscovery)

	// Temporarily block forever with a select statement (will be removed)
	select {}
}

// Function used to connect to a number of bootstrap peers
func connectToBootstrapPeers(ctx context.Context, host host.Host, bootstrapPeers []*peer.AddrInfo) error {
	// Keep track of the amount of successfully connected nodes
	connected := 0
	// Keep track of attempted connections made (unsuccessful or successful)
	attempted := 0

	// Make a channel for the result of each attempted connection
	success := make(chan bool)

	// Attempt connection to each bootstrap peer
	for _, peerInfo := range bootstrapPeers {
		go connectToBootstrapPeer(ctx, host, peerInfo, success)
	}

	// Wait for each connection to be attempted and count how many were successful
	for attempted < len(bootstrapPeers) {
		result := <-success
		if result {
			connected++
		}
		attempted++
	}

	// If the number of successful connection was zero, return an error
	if connected == 0 {
		return errors.New("did not successfully connect to any bootstrap peers")
	}

	// Otherwise at least one bootstrap peer was connected to, so even if any others failed, can safely ignore
	return nil
}

// Function used to connect to an individual peer
func connectToBootstrapPeer(ctx context.Context, host host.Host, peerAddr *peer.AddrInfo, success chan bool) {
	err := host.Connect(ctx, *peerAddr)
	if err != nil {
		// If connection errored, report this back to handler function
		success <- false
	} else {
		PeersMutex.Lock()
		// Connection successful so add peer to list of peers
		Peers = append(Peers, peerAddr)
		PeersMutex.Unlock()
		success <- true
	}
}

// Function used to discover other peers once connected to the bootstrap network
func discoverPeers(ctx context.Context, host host.Host, routingDiscovery *routing.RoutingDiscovery) {
	// Create a channel on which new peers will be discovered
	peerChan, err := routingDiscovery.FindPeers(ctx, protocol)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Infinitely loop waiting for a new peer to be discovered
	for peer := range peerChan {
		if peer.ID == host.ID() {
			continue
		}

		// Attempt a connection to the peer
		err := host.Connect(ctx, peer)
		if err != nil {
			fmt.Printf("Failed to connect to peer %s for reason %s", peer.ID, err)
		} else {
			// If connection successful add it to the list of peers
			PeersMutex.Lock()
			Peers = append(Peers, &peer)
			PeersMutex.Unlock()
		}
	}
}
