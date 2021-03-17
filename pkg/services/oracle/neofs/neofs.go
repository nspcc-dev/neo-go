package neofs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neofs-api-go/pkg/client"
	"github.com/nspcc-dev/neofs-api-go/pkg/container"
	"github.com/nspcc-dev/neofs-api-go/pkg/object"
	objectv2 "github.com/nspcc-dev/neofs-api-go/v2/object"
)

const (
	// URIScheme is the name of neofs URI scheme.
	URIScheme = "neofs"

	// containerIDSize is the size of container id in bytes.
	containerIDSize = sha256.Size
	// objectIDSize is the size of container id in bytes.
	objectIDSize = sha256.Size
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
// URI scheme is "neofs://<Container-ID>/<Object-ID/<Command>/<Params>".
// If Command is not provided, full object is requested.
func Get(ctx context.Context, priv *keys.PrivateKey, u *url.URL, addr string) ([]byte, error) {
	objectAddr, ps, err := parseNeoFSURL(u)
	if err != nil {
		return nil, err
	}

	c, err := client.New(&priv.PrivateKey, client.WithAddress(addr))
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

	ps := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(ps) == 0 || ps[0] == "" {
		return nil, nil, ErrMissingObject
	}

	containerID := container.NewID()
	if err := containerID.Parse(u.Hostname()); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidContainer, err)
	}

	objectID := object.NewID()
	if err := objectID.Parse(ps[0]); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidObject, err)
	}

	objectAddr := object.NewAddress()
	objectAddr.SetContainerID(containerID)
	objectAddr.SetObjectID(objectID)
	return objectAddr, ps[1:], nil
}

func getPayload(ctx context.Context, c *client.Client, addr *object.Address) ([]byte, error) {
	obj, err := c.GetObject(ctx, new(client.GetObjectParams).WithAddress(addr))
	if err != nil {
		return nil, err
	}
	return checkUTF8(obj.Payload())
}

func getRange(ctx context.Context, c *client.Client, addr *object.Address, ps ...string) ([]byte, error) {
	if len(ps) == 0 {
		return nil, ErrInvalidRange
	}
	r, err := parseRange(ps[0])
	if err != nil {
		return nil, err
	}
	data, err := c.ObjectPayloadRangeData(ctx, new(client.RangeDataParams).WithAddress(addr).WithRange(r))
	if err != nil {
		return nil, err
	}
	return checkUTF8(data)
}

func getHeader(ctx context.Context, c *client.Client, addr *object.Address) ([]byte, error) {
	obj, err := c.GetObjectHeader(ctx, new(client.ObjectHeaderParams).WithAddress(addr))
	if err != nil {
		return nil, err
	}
	msg := objectv2.ObjectToGRPCMessage(obj.ToV2()).Header
	b := bytes.NewBuffer(nil)
	err = new(jsonpb.Marshaler).Marshal(b, msg)
	return b.Bytes(), err
}

func getHash(ctx context.Context, c *client.Client, addr *object.Address, ps ...string) ([]byte, error) {
	if len(ps) == 0 || ps[0] == "" { // hash of the full payload
		obj, err := c.GetObjectHeader(ctx, new(client.ObjectHeaderParams).WithAddress(addr))
		if err != nil {
			return nil, err
		}
		return obj.PayloadChecksum().Sum(), nil
	}
	r, err := parseRange(ps[0])
	if err != nil {
		return nil, err
	}
	hashes, err := c.ObjectPayloadRangeSHA256(ctx,
		new(client.RangeChecksumParams).WithAddress(addr).WithRangeList(r))
	if err != nil {
		return nil, err
	}
	if len(hashes) == 0 {
		return nil, fmt.Errorf("%w: empty response", ErrInvalidRange)
	}
	return util.Uint256(hashes[0]).MarshalJSON()
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
