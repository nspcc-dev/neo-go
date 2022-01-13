package neofs

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
)

const (
	// URIScheme is the name of neofs URI scheme.
	URIScheme = "neofs"

	// rangeSep is a separator between offset and length.
	rangeSep = '|'

	rangeCmd  = "range"
	headerCmd = "header"
	hashCmd   = "hash"
)

// Various validation errors.
var (
	ErrInvalidScheme    = errors.New("invalid URI scheme")
	ErrMissingObject    = errors.New("object ID is missing from URI")
	ErrInvalidContainer = errors.New("container ID is invalid")
	ErrInvalidObject    = errors.New("object ID is invalid")
	ErrInvalidRange     = errors.New("object range is invalid (expected 'Offset|Length')")
	ErrInvalidCommand   = errors.New("invalid command")
)

// Get returns neofs object from the provided url.
// URI scheme is "neofs:<Container-ID>/<Object-ID/<Command>/<Params>".
// If Command is not provided, full object is requested.
func Get(ctx context.Context, priv *keys.PrivateKey, u *url.URL, addr string) ([]byte, error) {
	objectAddr, ps, err := parseNeoFSURL(u)
	if err != nil {
		return nil, err
	}

	c, err := client.New(
		client.WithDefaultPrivateKey(&priv.PrivateKey),
		client.WithURIAddress(addr, nil),
		client.WithNeoFSErrorParsing(),
	)
	if err != nil {
		return nil, err
	}

	switch {
	case len(ps) == 0 || ps[0] == "": // Get request
		return getPayload(ctx, c, objectAddr)
	case ps[0] == rangeCmd:
		return getRange(ctx, c, objectAddr, ps[1:]...)
	case ps[0] == headerCmd:
		return getHeader(ctx, c, objectAddr)
	case ps[0] == hashCmd:
		return getHash(ctx, c, objectAddr, ps[1:]...)
	default:
		return nil, ErrInvalidCommand
	}
}

// parseNeoFSURL returns parsed neofs address.
func parseNeoFSURL(u *url.URL) (*object.Address, []string, error) {
	if u.Scheme != URIScheme {
		return nil, nil, ErrInvalidScheme
	}

	ps := strings.Split(u.Opaque, "/")
	if len(ps) < 2 {
		return nil, nil, ErrMissingObject
	}

	containerID := cid.New()
	if err := containerID.Parse(ps[0]); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidContainer, err)
	}

	objectID := object.NewID()
	if err := objectID.Parse(ps[1]); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidObject, err)
	}

	objectAddr := object.NewAddress()
	objectAddr.SetContainerID(containerID)
	objectAddr.SetObjectID(objectID)
	return objectAddr, ps[2:], nil
}

func getPayload(ctx context.Context, c *client.Client, addr *object.Address) ([]byte, error) {
	res, err := c.GetObject(ctx, new(client.GetObjectParams).WithAddress(addr))
	if err != nil {
		return nil, err
	}
	return checkUTF8(res.Object().Payload())
}

func getRange(ctx context.Context, c *client.Client, addr *object.Address, ps ...string) ([]byte, error) {
	if len(ps) == 0 {
		return nil, ErrInvalidRange
	}
	r, err := parseRange(ps[0])
	if err != nil {
		return nil, err
	}
	res, err := c.ObjectPayloadRangeData(ctx, new(client.RangeDataParams).WithAddress(addr).WithRange(r))
	if err != nil {
		return nil, err
	}
	return checkUTF8(res.Data())
}

func getHeader(ctx context.Context, c *client.Client, addr *object.Address) ([]byte, error) {
	res, err := c.HeadObject(ctx, new(client.ObjectHeaderParams).WithAddress(addr))
	if err != nil {
		return nil, err
	}
	return res.Object().MarshalHeaderJSON()
}

func getHash(ctx context.Context, c *client.Client, addr *object.Address, ps ...string) ([]byte, error) {
	if len(ps) == 0 || ps[0] == "" { // hash of the full payload
		res, err := c.HeadObject(ctx, new(client.ObjectHeaderParams).WithAddress(addr))
		if err != nil {
			return nil, err
		}
		return res.Object().PayloadChecksum().Sum(), nil
	}
	r, err := parseRange(ps[0])
	if err != nil {
		return nil, err
	}
	res, err := c.HashObjectPayloadRanges(ctx,
		new(client.RangeChecksumParams).WithAddress(addr).WithRangeList(r))
	if err != nil {
		return nil, err
	}
	hashes := res.Hashes()
	if len(hashes) == 0 {
		return nil, fmt.Errorf("%w: empty response", ErrInvalidRange)
	}
	u256, err := util.Uint256DecodeBytesBE(hashes[0])
	if err != nil {
		return nil, fmt.Errorf("decode Uint256: %w", err)
	}
	return u256.MarshalJSON()
}

func parseRange(s string) (*object.Range, error) {
	sepIndex := strings.IndexByte(s, rangeSep)
	if sepIndex < 0 {
		return nil, ErrInvalidRange
	}
	offset, err := strconv.ParseUint(s[:sepIndex], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid offset", ErrInvalidRange)
	}
	length, err := strconv.ParseUint(s[sepIndex+1:], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid length", ErrInvalidRange)
	}

	r := object.NewRange()
	r.SetOffset(offset)
	r.SetLength(length)
	return r, nil
}

func checkUTF8(v []byte) ([]byte, error) {
	if !utf8.Valid(v) {
		return nil, errors.New("invalid UTF-8")
	}
	return v, nil
}
