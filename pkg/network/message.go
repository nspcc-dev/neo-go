package network

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

// Values used for the magic field, according to the docs.
const (
	ModeMainNet = 0x00746e41 // 7630401
	ModeTestNet = 0x74746e41 // 1953787457
	// ModeDevNet  = 0xDEADBEAF
)

// Message is the complete message send between nodes.
//
//	Size	Field		DataType		Description
// 	------------------------------------------------------
//	4		Magic		uint32			Protocol ID
//	12		Command		char[12]		Command
//	4		length		uint32			Length of payload
//	4		Checksum	uint32			Checksum
//	length	Payload		uint8[length]	Content of message
type Message struct {
	Magic uint32
	// Command is utf8 code, of which the length is 12 bytes,
	// the extra part is filled with 0.
	Command []byte
	// Length of the payload
	Length uint32
	// Checksum is the first 4 bytes of the value that two times SHA256
	// hash of the payload
	Checksum uint32
	// Payload send with the message.
	Payload []byte
}

type commandType string

// valid commands used to send between nodes.
const (
	cmdVersion    commandType = "version"
	cmdVerack                 = "verack"
	cmdGetAddr                = "getaddr"
	cmdAddr                   = "addr"
	cmdGetHeaders             = "getheaders"
	cmdHeaders                = "headers"
	cmdGetBlocks              = "getblocks"
	cmdInv                    = "inv"
	cmdGetData                = "getdata"
	cmdBlock                  = "block"
	cmdTX                     = "tx"
)

func newMessage(magic uint32, cmd commandType, payload []byte) *Message {
	sum := sumSHA256(sumSHA256(payload))[:4]
	sumuint32 := binary.LittleEndian.Uint32(sum)

	return &Message{
		Magic:    magic,
		Command:  cmdToByteSlice(cmd),
		Length:   uint32(len(payload)),
		Checksum: sumuint32,
		Payload:  payload,
	}
}

// Converts the 12 byte command slice to a commandType.
func (m *Message) commandType() commandType {
	cmd := string(bytes.TrimRight(m.Command, "\x00"))
	switch cmd {
	case "version":
		return cmdVersion
	case "verack":
		return cmdVerack
	case "addr":
		return cmdAddr
	case "getheaders":
		return cmdGetHeaders
	case "header":
		return cmdHeaders
	case "getblocks":
		return cmdGetBlocks
	case "inv":
		return cmdInv
	case "getdata":
		return cmdGetData
	case "block":
		return cmdBlock
	case "tx":
		return cmdTX
	default:
		return ""
	}
}

// decode a Message from the given reader.
func (m *Message) decode(r io.Reader) error {
	// 24 bytes for the fixed sized fields.
	buf := make([]byte, 24)
	if _, err := r.Read(buf); err != nil {
		return err
	}

	m.Magic = binary.LittleEndian.Uint32(buf[0:4])
	m.Command = buf[4:16]
	m.Length = binary.LittleEndian.Uint32(buf[16:20])
	m.Checksum = binary.LittleEndian.Uint32(buf[20:24])

	payload := make([]byte, m.Length)
	if _, err := r.Read(payload); err != nil {
		return err
	}

	// Compare the checksum of the payload.
	if !compareChecksum(m.Checksum, payload) {
		return errors.New("checksum mismatch error")
	}

	m.Payload = payload

	return nil
}

// encode a Message to any given io.Writer.
func (m *Message) encode(w io.Writer) error {
	// 24 bytes for the fixed sized fields + the length of the payload.
	buf := make([]byte, 24+m.Length)

	binary.LittleEndian.PutUint32(buf[0:4], m.Magic)
	copy(buf[4:16], m.Command)
	binary.LittleEndian.PutUint32(buf[16:20], m.Length)
	binary.LittleEndian.PutUint32(buf[20:24], m.Checksum)
	copy(buf[24:len(buf)], m.Payload)

	_, err := w.Write(buf)
	return err
}

func (m *Message) decodePayload() (interface{}, error) {
	switch m.commandType() {
	case cmdVersion:
		v := &Version{}
		if err := v.decode(m.Payload); err != nil {
			return nil, err
		}
		return v, nil
	}
	return nil, nil
}

// Version payload description.
//
//	Size	Field		  DataType		Description
// 	---------------------------------------------------------------------------------------------
// 	4		Version		  uint32		Version of protocol, 0 for now
// 	8		Services	  uint64		The service provided by the node is currently 1
// 	4		Timestamp	  uint32		Current time
// 	2		Port		  uint16		Port that the server is listening on, it's 0 if not used.
// 	4		Nonce		  uint32		It's used to distinguish the node from public IP
// 	?		UserAgent	  varstr		Client ID
// 	4		StartHeight	  uint32		Height of block chain
// 	1		Relay		  bool			Whether to receive and forward
type Version struct {
	// currently the version of the protocol is 0
	Version uint32
	// currently 1
	Services uint64
	// timestamp
	Timestamp uint32
	// port this server is listening on
	Port uint16
	// it's used to distinguish the node from public IP
	Nonce uint32
	// client id
	UserAgent []byte // ?
	// Height of the block chain
	StartHeight uint32
	// Whether to receive and forward
	Relay bool
}

func newVersionPayload(p uint16, ua string, h uint32, r bool) *Version {
	return &Version{
		Version:     0,
		Services:    1,
		Timestamp:   12345,
		Port:        p,
		Nonce:       1911099534,
		UserAgent:   []byte(ua),
		StartHeight: 0,
		Relay:       r,
	}
}

func (p *Version) decode(b []byte) error {
	// Fixed fields have a total of 27 bytes. We substract this size
	// with the total buffer length to know the length of the user agent.
	lenUA := len(b) - 27

	p.Version = binary.LittleEndian.Uint32(b[0:4])
	p.Services = binary.LittleEndian.Uint64(b[4:12])
	p.Timestamp = binary.LittleEndian.Uint32(b[12:16])
	// FIXME: port's byteorder should be big endian according to the docs.
	// but when connecting to the privnet docker image it's little endian.
	p.Port = binary.LittleEndian.Uint16(b[16:18])
	p.Nonce = binary.LittleEndian.Uint32(b[18:22])
	p.UserAgent = b[22 : 22+lenUA]
	curlen := 22 + lenUA
	p.StartHeight = binary.LittleEndian.Uint32(b[curlen : curlen+4])
	p.Relay = b[len(b)-1 : len(b)][0] == 1

	return nil
}

func (p *Version) encode() ([]byte, error) {
	// 27 bytes for the fixed size fields + the length of the user agent
	// which is kinda variable, according to the docs.
	buf := make([]byte, 27+len(p.UserAgent))

	binary.LittleEndian.PutUint32(buf[0:4], p.Version)
	binary.LittleEndian.PutUint64(buf[4:12], p.Services)
	binary.LittleEndian.PutUint32(buf[12:16], p.Timestamp)
	// FIXME: byte order (little / big)?
	binary.LittleEndian.PutUint16(buf[16:18], p.Port)
	binary.LittleEndian.PutUint32(buf[18:22], p.Nonce)
	copy(buf[22:22+len(p.UserAgent)], p.UserAgent) //
	curLen := 22 + len(p.UserAgent)
	binary.LittleEndian.PutUint32(buf[curLen:curLen+4], p.StartHeight)

	// yikes
	var b []byte
	if p.Relay {
		b = []byte{1}
	} else {
		b = []byte{0}
	}

	copy(buf[curLen+4:len(buf)], b)

	return buf, nil
}

// convert a command (string) to a byte slice filled with 0 bytes till
// size 12.
func cmdToByteSlice(cmd commandType) []byte {
	cmdLen := len(cmd)
	if cmdLen > 12 {
		panic("exceeded command max length of size 12")
	}

	// The command can have max 12 bytes, rest is filled with 0.
	b := []byte(cmd)
	for i := 0; i < 12-cmdLen; i++ {
		b = append(b, byte('\x00'))
	}

	return b
}

func sumSHA256(b []byte) []byte {
	h := sha256.New()
	h.Write(b)
	return h.Sum(nil)
}

func compareChecksum(have uint32, b []byte) bool {
	sum := sumSHA256(sumSHA256(b))[:4]
	want := binary.LittleEndian.Uint32(sum)
	return have == want
}
