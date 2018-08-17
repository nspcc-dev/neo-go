# Package - Peer



## Responsibility
// TODO
Once a connection has been made. The connection will represent a established peer to the localNode. Since a connection is a golang primitive, that we cannot do much with. The peer package will encapsulate the connection, while adding extra functionality.




## Usage

//TODO


### Temp notes

A peer should contain:

- connection

- address

-  nonce/ID

- CreatedAt (Time when connection was made)

Functions:

- Protocol Negotiation

- ReadMSG

- WriteMSG

- MetaData

- IsInbound


Should we follow the actor model for Peers? Each Peer will have a ID, which we can take as the PID or if
we launch a go-routine for each peer, then we can use that as an implicit PID.

Peer information should be stored into a database, if no db exists, we should get it from an initial peers file.
We can use this to periodically store information about a peer.



What are the reasons to disconnect a peer?

- Taking too long to send me back a response: Detector added
- Have not heard anything from peer in a while; put a timer on the queue that handles incoming messages
- Malicious behaviour: This is quite wide and would need more defining.


TODO: make sure that the peer is properly disconnected and that the channels are garbage collected