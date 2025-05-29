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
	"time"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	neofscrypto "github.com/nspcc-dev/neofs-sdk-go/crypto"
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

// Client is a NeoFS client interface.
type Client interface {
	SearchObjects(ctx context.Context, cnr cid.ID, filters object.SearchFilters, attrs []string, cursor string, signer neofscrypto.Signer, opts client.SearchObjectsOptions) ([]client.SearchResultItem, string, error)
	ObjectGetInit(ctx context.Context, container cid.ID, id oid.ID, s user.Signer, get client.PrmObjectGet) (object.Object, *client.PayloadReader, error)
	ObjectRangeInit(ctx context.Context, container cid.ID, id oid.ID, offset uint64, length uint64, s user.Signer, objectRange client.PrmObjectRange) (*client.ObjectRangeReader, error)
	ObjectHead(ctx context.Context, containerID cid.ID, objectID oid.ID, signer user.Signer, prm client.PrmObjectHead) (*object.Object, error)
	ObjectHash(ctx context.Context, containerID cid.ID, objectID oid.ID, signer user.Signer, prm client.PrmObjectHash) ([][]byte, error)
	Close() error
}

// Get returns a neofs object from the provided url.
// URI scheme is "neofs:<Container-ID>/<Object-ID/<Command>/<Params>".
// If Command is not provided, full object is requested.
func Get(ctx context.Context, priv *keys.PrivateKey, u *url.URL, addr string) (io.ReadCloser, error) {
	c, err := GetClient(ctx, addr, 0)
	if err != nil {
		return clientCloseWrapper{c: c}, fmt.Errorf("failed to create client: %w", err)
	}
	return GetWithClient(ctx, c, priv, u, true)
}

// GetWithClient returns a neofs object from the provided url using the provided client.
// URI scheme is "neofs:<Container-ID>/<Object-ID/<Command>/<Params>".
// If Command is not provided, full object is requested. If wrapClientCloser is true,
// the client will be closed when the returned ReadCloser is closed.
func GetWithClient(ctx context.Context, c Client, priv *keys.PrivateKey, u *url.URL, wrapClientCloser bool) (io.ReadCloser, error) {
	objectAddr, ps, err := parseNeoFSURL(u)
	if err != nil {
		return nil, err
	}
	var (
		res io.ReadCloser
		s   = user.NewAutoIDSignerRFC6979(priv.PrivateKey)
	)
	switch {
	case len(ps) == 0 || ps[0] == "":
		res, err = getPayload(ctx, s, c, objectAddr)
	case ps[0] == rangeCmd:
		res, err = getRange(ctx, s, c, objectAddr, ps[1:]...)
	case ps[0] == headerCmd:
		res, err = getHeader(ctx, s, c, objectAddr)
	case ps[0] == hashCmd:
		res, err = getHash(ctx, s, c, objectAddr, ps[1:]...)
	default:
		return nil, ErrInvalidCommand
	}
	if err != nil {
		return nil, err
	}
	if wrapClientCloser {
		return clientCloseWrapper{
			c:          c,
			ReadCloser: res,
		}, nil
	}
	return res, nil
}

type clientCloseWrapper struct {
	io.ReadCloser
	c Client
}

func (w clientCloseWrapper) Close() error {
	var res error
	if w.ReadCloser != nil {
		res = w.ReadCloser.Close()
	}
	if w.c != nil {
		closeErr := w.c.Close()
		if closeErr != nil && res == nil {
			res = closeErr
		}
	}
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
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidContainer, err)
	}

	var objectID oid.ID
	if err := objectID.DecodeString(ps[1]); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidObject, err)
	}
	var objAddr = new(oid.Address)
	objAddr.SetContainer(containerID)
	objAddr.SetObject(objectID)
	return objAddr, ps[2:], nil
}

func getPayload(ctx context.Context, s user.Signer, c Client, addr *oid.Address) (io.ReadCloser, error) {
	var iorc io.ReadCloser
	_, rc, err := c.ObjectGetInit(ctx, addr.Container(), addr.Object(), s, client.PrmObjectGet{})
	if rc != nil {
		iorc = rc
	}
	return iorc, err
}

func getRange(ctx context.Context, s user.Signer, c Client, addr *oid.Address, ps ...string) (io.ReadCloser, error) {
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

func getObjHeader(ctx context.Context, s user.Signer, c Client, addr *oid.Address) (*object.Object, error) {
	return c.ObjectHead(ctx, addr.Container(), addr.Object(), s, client.PrmObjectHead{})
}

func getHeader(ctx context.Context, s user.Signer, c Client, addr *oid.Address) (io.ReadCloser, error) {
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

func getHash(ctx context.Context, s user.Signer, c Client, addr *oid.Address, ps ...string) (io.ReadCloser, error) {
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

// ObjectSearch returns a channel of object search results from the provided container.
func ObjectSearch(ctx context.Context, c Client, priv *keys.PrivateKey, containerID cid.ID, filters object.SearchFilters, attrs []string) (<-chan client.SearchResultItem, <-chan error) {
	out := make(chan client.SearchResultItem)
	errChan := make(chan error)

	go func() {
		defer close(out)
		defer close(errChan)
		var (
			s      = user.NewAutoIDSignerRFC6979(priv.PrivateKey)
			basic  = BasicService{Ctx: ctx}
			cursor = ""
		)

		for {
			var (
				page       []client.SearchResultItem
				nextCursor string
			)

			err := basic.Retry(func() error {
				var err error
				page, nextCursor, err = c.SearchObjects(ctx, containerID, filters, attrs, cursor, s, client.SearchObjectsOptions{})
				if err != nil {
					return fmt.Errorf("failed to search objects: %w", err)
				}
				return nil
			})

			if err != nil {
				errChan <- err
				return
			}

			for _, itm := range page {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				case out <- itm:
				}
			}

			if nextCursor == "" {
				return
			}
			cursor = nextCursor
		}
	}()
	return out, errChan
}

// GetClient returns a NeoFS client configured with the specified address and context.
// If timeout is 0, the default timeout will be used.
func GetClient(ctx context.Context, addr string, timeout time.Duration) (*client.Client, error) {
	var prmDial client.PrmDial
	if addr == "" {
		return nil, errors.New("address is empty")
	}
	prmDial.SetServerURI(addr)
	prmDial.SetContext(ctx)
	if timeout != 0 {
		prmDial.SetTimeout(timeout)
		prmDial.SetStreamTimeout(timeout)
	}
	c, err := client.New(client.PrmInit{})
	if err != nil {
		return nil, fmt.Errorf("can't create NeoFS client: %w", err)
	}

	if err := c.Dial(prmDial); err != nil {
		return nil, fmt.Errorf("can't init NeoFS client: %w", err)
	}

	return c, nil
}
