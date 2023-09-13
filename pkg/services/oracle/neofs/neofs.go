package neofs

import (
	"bytes"
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
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/user"
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

// Get returns a neofs object from the provided url.
// URI scheme is "neofs:<Container-ID>/<Object-ID/<Command>/<Params>".
// If Command is not provided, full object is requested.
func Get(ctx context.Context, priv *keys.PrivateKey, u *url.URL, addr string) (io.ReadCloser, error) {
	objectAddr, ps, err := parseNeoFSURL(u)
	if err != nil {
		return nil, err
	}

	c, err := client.New(client.PrmInit{})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	var (
		res  = clientCloseWrapper{c: c}
		prmd client.PrmDial
	)
	prmd.SetServerURI(addr)
	prmd.SetContext(ctx)
	err = c.Dial(prmd) //nolint:contextcheck // contextcheck: Function `Dial->Balance->SendUnary->Init->setNeoFSAPIServer` should pass the context parameter
	if err != nil {
		return res, err
	}

	var s = user.NewAutoIDSignerRFC6979(priv.PrivateKey)
	switch {
	case len(ps) == 0 || ps[0] == "": // Get request
		res.ReadCloser, err = getPayload(ctx, s, c, objectAddr)
	case ps[0] == rangeCmd:
		res.ReadCloser, err = getRange(ctx, s, c, objectAddr, ps[1:]...)
	case ps[0] == headerCmd:
		res.ReadCloser, err = getHeader(ctx, s, c, objectAddr)
	case ps[0] == hashCmd:
		res.ReadCloser, err = getHash(ctx, s, c, objectAddr, ps[1:]...)
	default:
		err = ErrInvalidCommand
	}
	return res, err
}

type clientCloseWrapper struct {
	io.ReadCloser
	c *client.Client
}

func (w clientCloseWrapper) Close() error {
	var res error
	if w.ReadCloser != nil {
		res = w.ReadCloser.Close()
	}
	w.c.Close()
	return res
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

func getPayload(ctx context.Context, s user.Signer, c *client.Client, addr *oid.Address) (io.ReadCloser, error) {
	var iorc io.ReadCloser
	_, rc, err := c.ObjectGetInit(ctx, addr.Container(), addr.Object(), s, client.PrmObjectGet{})
	if rc != nil {
		iorc = rc
	}
	return iorc, err
}

func getRange(ctx context.Context, s user.Signer, c *client.Client, addr *oid.Address, ps ...string) (io.ReadCloser, error) {
	var iorc io.ReadCloser
	if len(ps) == 0 {
		return nil, ErrInvalidRange
	}
	r, err := parseRange(ps[0])
	if err != nil {
		return nil, err
	}

	rc, err := c.ObjectRangeInit(ctx, addr.Container(), addr.Object(), r.GetOffset(), r.GetLength(), s, client.PrmObjectRange{})
	if rc != nil {
		iorc = rc
	}
	return iorc, err
}

func getObjHeader(ctx context.Context, s user.Signer, c *client.Client, addr *oid.Address) (*object.Object, error) {
	return c.ObjectHead(ctx, addr.Container(), addr.Object(), s, client.PrmObjectHead{})
}

func getHeader(ctx context.Context, s user.Signer, c *client.Client, addr *oid.Address) (io.ReadCloser, error) {
	obj, err := getObjHeader(ctx, s, c, addr)
	if err != nil {
		return nil, err
	}
	res, err := obj.MarshalHeaderJSON()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(res)), nil
}

func getHash(ctx context.Context, s user.Signer, c *client.Client, addr *oid.Address, ps ...string) (io.ReadCloser, error) {
	if len(ps) == 0 || ps[0] == "" { // hash of the full payload
		obj, err := getObjHeader(ctx, s, c, addr)
		if err != nil {
			return nil, err
		}
		sum, flag := obj.PayloadChecksum()
		if !flag {
			return nil, errors.New("missing checksum in the reply")
		}
		return io.NopCloser(bytes.NewReader(sum.Value())), nil
	}
	r, err := parseRange(ps[0])
	if err != nil {
		return nil, err
	}
	var hashPrm client.PrmObjectHash
	hashPrm.SetRangeList(r.GetOffset(), r.GetLength())

	hashes, err := c.ObjectHash(ctx, addr.Container(), addr.Object(), s, hashPrm)
	if err != nil {
		return nil, err
	}
	if len(hashes) == 0 {
		return nil, fmt.Errorf("%w: empty response", ErrInvalidRange)
	}
	u256, err := util.Uint256DecodeBytesBE(hashes[0])
	if err != nil {
		return nil, fmt.Errorf("decode Uint256: %w", err)
	}
	res, err := u256.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(res)), nil
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
