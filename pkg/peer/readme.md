# Package - Peer



## Responsibility

Once a connection has been made. The connection will represent a established peer to the localNode. Since a connection  and the `Wire` is a golang primitive, that we cannot do much with. The peer package will encapsulate both, while adding extra functionality.


## Features

- The handshake protocol is automatically executed and handled by the peer package. If a Version/Verack is received twice, the peer will be disconnected.

- IdleTimeouts: If a Message is not received from the peer within a set period of time, the peer will be disconnected.

- StallTimeouts: For Example, If a GetHeaders, is sent to the Peer and a Headers Response is not received within a certain period of time, then the peer is disconnected.

- Concurrency Model: The concurrency model used is similar to Actor model, with a few changes. Messages can be sent to a peer asynchronously or synchronously. An example of an synchornous message send is the `RequestHeaders` method, where the channel blocks until an error value is received. The `OnHeaders` message is however asynchronously called. Furthermore, all methods passed through the config, are wrapped inside of an additional `Peers` method, this is to lay the ground work to capturing statistics regarding a specific command. These are also used so that we can pass behaviour to be executed down the channel.

- Configuration: Each Peer will have a config struct passed to it, with information about the Local Peer and functions that will encapsulate the behaviour of what the peer should do, given a request. This way, the peer is not dependent on any other package.

## Usage

	conn, err := net.Dial("tcp", "seed2.neo.org:10333")
	if err != nil {
		fmt.Println("Error dialing connection", err.Error())
		return
	}

	config := peer.LocalConfig{
		Net:         protocol.MainNet,
		UserAgent:   "NEO-G",
		Services:    protocol.NodePeerService,
		Nonce:       1200,
		ProtocolVer: 0,
		Relay:       false,
		Port:        10332,
		StartHeight: LocalHeight,
		OnHeader:    OnHeader,
	}

	p := peer.NewPeer(conn, false, config)
	err = p.Run()

	hash, err := util.Uint256DecodeString(chainparams.GenesisHash)
	// hash2, err := util.Uint256DecodeString("ff8fe95efc5d1cc3a22b17503aecaf289cef68f94b79ddad6f613569ca2342d8")
	err = p.RequestHeaders(hash)

    func OnHeader(peer *peer.Peer, msg *payload.HeadersMessage) {
        // This function is passed to peer
        // and the peer will execute it on receiving a header
    }

    func LocalHeight() uint32 {
        // This will be a function from the object that handles the block heights
	    return 10
    }


### Notes


Should we follow the actor model for Peers? Each Peer will have a ID, which we can take as the PID or if
we launch a go-routine for each peer, then we can use that as an implicit PID.

Peer information should be stored into a database, if no db exists, we should get it from an initial peers file.
We can use this to periodically store information about a peer.