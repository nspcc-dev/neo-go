# Package - Wire


The neo wire package will implement the network protocol displayed here: http://docs.neo.org/en-us/network/network-protocol.html

This package will act as a standalone package.

# Responsibility

This package will solely be responsible for Encoding and decoding a Message.
It will return a Messager interface, which means that the caller of the package will need to type assert it to the appropriate type.

# Usage 

## Write Message 

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	message, err := NewVersionMessage(tcpAddrMe, 0, true, defaultVersion)

	conn := new(bytes.Buffer)
	if err := WriteMessage(con, Production, message); err != nil {
		// handle Error
	}

## Read Message 

	readmsg, err := ReadMessage(conn, Production)

	if err != nil {
		// Log error
	}
	version := readmsg.(*VersionMessage)

## RoadMap 

These below commands are left to implement.

	[ x ] CMDVersion (Tests added)
	[ x ] CMDVerack (Tests Added)
	[ x ] CMDGetAddr(Tests Added)
	[ x ] CMDAddr (Tests Added)
	[ x ] CMDGetHeaders (Tests Added)
	[ x ] CMDHeaders (Tests Added)
	[ x ] CMDGetBlocks (Tests Added)
	[ x ] CMDInv (Tests Added)
	[ x ] CMDGetData (Tests Added)
	[ x ] CMDBlock (Tests Added)
	[ x ] CMDTX // Each tx implments the messager interface
	[   ] CMDConsensus

## Notes 

Please not that this package will do sanity checks on the fields, however it will not verify if any of the items are valid for the current state of the system. Please see `Responbilities`.

# Contributors

When modifying this package, please ensure that it does not depend on any other package and that it conforms to the Single Responsibility Principle. If you see somewhere in the current implementation that does not do this, then please tell me.

