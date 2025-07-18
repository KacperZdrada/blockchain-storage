package network

import (
	"context"
	"errors"
	"fmt"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"math/rand"
	"time"
)

func StartNode(ctx context.Context, port int, bootstrapAddr string) (host.Host, *dht.IpfsDHT, error) {
	// Generate a key pair for the node's identity
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create a libp2p node
	host, err := libp2p.New(libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)), libp2p.Identity(priv))
	if err != nil {
		return nil, nil, err
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
			return nil, nil, err
		}

		// Get peer ID and address
		peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return nil, nil, err
		}

		// Add the peer info to list of bootstrap peers
		bootstrapPeers = append(bootstrapPeers, peerInfo)
	}

	// Check if bootstrap peers provided and if so connect to them
	if len(bootstrapPeers) > 0 {
		err := connectToBootstrapPeers(ctx, host, bootstrapPeers)
		if err != nil {
			return nil, nil, err
		}
	}

	// Create a helper discovery object with the local DHT as its routing system
	// It acts as a high-level API for discovery operations with the DHT
	routingDiscovery := routing.NewRoutingDiscovery(localDHT)

	// Advertise that the newly created node is accepting requests on the provided protocol
	util.Advertise(ctx, routingDiscovery, protocol)

	// TODO: Advertise any saved files

	// Attempt to discover other peers
	go discoverPeers(ctx, host, routingDiscovery)

	return host, localDHT, nil
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
		}
	}
}

// ProvideContent is a function used to advertise to the P2P network that the host holds a certain file
func ProvideContent(ctx context.Context, DHT *dht.IpfsDHT, merkleRoot []byte) (cid.Cid, error) {
	// Create the content ID from the merkleRoot of the file being stored
	hash, err := multihash.Sum(merkleRoot, multihash.SHA2_256, -1)
	if err != nil {
		return cid.Undef, err
	}
	contentCid := cid.NewCidV1(cid.Raw, hash)

	// Advertise it to the network
	routingDiscovery := routing.NewRoutingDiscovery(DHT)
	util.Advertise(ctx, routingDiscovery, contentCid.String())

	return contentCid, nil
}

// FindProviders will find any nodes that are able to provide the file based on the contentID
func FindProviders(ctx context.Context, host host.Host, DHT *dht.IpfsDHT, contentCid cid.Cid) ([]peer.AddrInfo, error) {
	// Create a channel on which found providers will be sent
	peers := DHT.FindProvidersAsync(ctx, contentCid, 5)
	var providers []peer.AddrInfo

	// Wait on providers and add to the list once found
	for peer := range peers {
		if peer.ID == host.ID() {
			continue
		}
		providers = append(providers, peer)
	}
	if len(providers) == 0 {
		return nil, errors.New("could not find any providers")
	}
	return providers, nil
}

// SelectRandomPeers randomly selects the replication factor number of nodes from all connected nodes
func SelectRandomPeers(allPeers []peer.ID, number int) ([]peer.ID, error) {
	connected := len(allPeers)
	// Do some error checking with regards to peers connected and the replication factor
	if connected == 0 {
		return nil, errors.New("no peers to select")
	}
	if connected < number {
		fmt.Printf("not enough peers to achieve replication factor: %d connected peers, %d replication factor. Replicating to the %d peers only.", connected, number, connected)
		number = len(allPeers)
	}

	var selectedPeers []peer.ID
	selectedIndex := make(map[int]bool)
	randomGenerator := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate a random index and check if it has not already been generated before selecting a peer
	for len(selectedPeers) < number {
		index := randomGenerator.Intn(connected)
		if selectedIndex[index] {
			continue
		} else {
			selectedIndex[index] = true
			selectedPeers = append(selectedPeers, allPeers[index])
		}
	}

	return selectedPeers, nil
}
