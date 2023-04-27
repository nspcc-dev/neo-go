package neofs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	neofsecdsa "github.com/nspcc-dev/neofs-sdk-go/crypto/ecdsa"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
)

// ResultReader is a function that reads required amount of data and
// checks it.
type ResultReader func(io.Reader) ([]byte, error)

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

// Get returns a neofs object from the provided url.
// URI scheme is "neofs:<Container-ID>/<Object-ID/<Command>/<Params>".
// If Command is not provided, full object is requested.
func Get(ctx context.Context, priv *keys.PrivateKey, u *url.URL, addr string, resReader ResultReader) ([]byte, error) {
	objectAddr, ps, err := parseNeoFSURL(u)
	if err != nil {
		return nil, err
	}

	var c = new(client.Client)
	var prmi client.PrmInit
	prmi.ResolveNeoFSFailures()
	prmi.SetDefaultSigner(neofsecdsa.Signer(priv.PrivateKey))
	c.Init(prmi)

	var prmd client.PrmDial
	prmd.SetServerURI(addr)
	prmd.SetContext(ctx)
	err = c.Dial(prmd) //nolint:contextcheck // contextcheck: Function `Dial->Balance->SendUnary->Init->setNeoFSAPIServer` should pass the context parameter
	if err != nil {
		return nil, err
	}
	defer c.Close()

	switch {
	case len(ps) == 0 || ps[0] == "": // Get request
		return getPayload(ctx, c, objectAddr, resReader)
	case ps[0] == rangeCmd:
		return getRange(ctx, c, objectAddr, resReader, ps[1:]...)
	case ps[0] == headerCmd:
		return getHeader(ctx, c, objectAddr)
	case ps[0] == hashCmd:
		return getHash(ctx, c, objectAddr, ps[1:]...)
	default:
		return nil, ErrInvalidCommand
	}
}

// parseNeoFSURL returns parsed neofs address.
func parseNeoFSURL(u *url.URL) (*oid.Address, []string, error) {
	if u.Scheme != URIScheme {
		return nil, nil, ErrInvalidScheme
	}

	ps := strings.Split(u.Opaque, "/")
	if len(ps) < 2 {
		return nil, nil, ErrMissingObject
	}

	var containerID cid.ID
	if err := containerID.DecodeString(ps[0]); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidContainer, err) //nolint:errorlint // errorlint: non-wrapping format verb for fmt.Errorf. Use `%w` to format errors
	}

	var objectID oid.ID
	if err := objectID.DecodeString(ps[1]); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidObject, err) //nolint:errorlint // errorlint: non-wrapping format verb for fmt.Errorf. Use `%w` to format errors
	}
	var objAddr = new(oid.Address)
	objAddr.SetContainer(containerID)
	objAddr.SetObject(objectID)
	return objAddr, ps[2:], nil
}

func getPayload(ctx context.Context, c *client.Client, addr *oid.Address, resReader ResultReader) ([]byte, error) {
	var getPrm client.PrmObjectGet
	getPrm.FromContainer(addr.Container())
	getPrm.ByID(addr.Object())

	objR, err := c.ObjectGetInit(ctx, getPrm)
	if err != nil {
		return nil, err
	}
	resp, err := resReader(objR)
	if err != nil {
		return nil, err
	}
	_, err = objR.Close() // Using ResolveNeoFSFailures.
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func getRange(ctx context.Context, c *client.Client, addr *oid.Address, resReader ResultReader, ps ...string) ([]byte, error) {
	if len(ps) == 0 {
		return nil, ErrInvalidRange
	}
	r, err := parseRange(ps[0])
	if err != nil {
		return nil, err
	}
	var rangePrm client.PrmObjectRange
	rangePrm.FromContainer(addr.Container())
	rangePrm.ByID(addr.Object())
	rangePrm.SetLength(r.GetLength())
	rangePrm.SetOffset(r.GetOffset())

	rangeR, err := c.ObjectRangeInit(ctx, rangePrm)
	if err != nil {
		return nil, err
	}
	resp, err := resReader(rangeR)
	if err != nil {
		return nil, err
	}
	_, err = rangeR.Close() // Using ResolveNeoFSFailures.
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func getObjHeader(ctx context.Context, c *client.Client, addr *oid.Address) (*object.Object, error) {
	var headPrm client.PrmObjectHead
	headPrm.FromContainer(addr.Container())
	headPrm.ByID(addr.Object())

	res, err := c.ObjectHead(ctx, headPrm)
	if err != nil {
		return nil, err
	}
	var obj = object.New()
	if !res.ReadHeader(obj) {
		return nil, errors.New("missing header in the reply")
	}
	return obj, nil
}

func getHeader(ctx context.Context, c *client.Client, addr *oid.Address) ([]byte, error) {
	obj, err := getObjHeader(ctx, c, addr)
	if err != nil {
		return nil, err
	}
	return obj.MarshalHeaderJSON()
}

func getHash(ctx context.Context, c *client.Client, addr *oid.Address, ps ...string) ([]byte, error) {
	if len(ps) == 0 || ps[0] == "" { // hash of the full payload
		obj, err := getObjHeader(ctx, c, addr)
		if err != nil {
			return nil, err
		}
		sum, flag := obj.PayloadChecksum()
		if !flag {
			return nil, errors.New("missing checksum in the reply")
		}
		return sum.Value(), nil
	}
	r, err := parseRange(ps[0])
	if err != nil {
		return nil, err
	}
	var hashPrm client.PrmObjectHash
	hashPrm.FromContainer(addr.Container())
	hashPrm.ByID(addr.Object())
	hashPrm.SetRangeList(r.GetOffset(), r.GetLength())

	res, err := c.ObjectHash(ctx, hashPrm)
	if err != nil {
		return nil, err
	}
	hashes := res.Checksums() // Using ResolveNeoFSFailures.
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
