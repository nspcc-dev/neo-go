package smartcontract

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"gopkg.in/yaml.v2"
)

type permission manifest.Permission

const (
	permHashKey   = "hash"
	permGroupKey  = "group"
	permMethodKey = "methods"
)

func (p permission) MarshalYAML() (interface{}, error) {
	m := make(yaml.MapSlice, 0, 2)
	switch p.Contract.Type {
	case manifest.PermissionWildcard:
	case manifest.PermissionHash:
		m = append(m, yaml.MapItem{
			Key:   permHashKey,
			Value: p.Contract.Value.(util.Uint160).StringLE(),
		})
	case manifest.PermissionGroup:
		bs := p.Contract.Value.(*keys.PublicKey).Bytes()
		m = append(m, yaml.MapItem{
			Key:   permGroupKey,
			Value: hex.EncodeToString(bs),
		})
	default:
		return nil, fmt.Errorf("invalid permission type: %d", p.Contract.Type)
	}

	var val interface{} = "*"
	if !p.Methods.IsWildcard() {
		val = p.Methods.Value
	}

	m = append(m, yaml.MapItem{
		Key:   permMethodKey,
		Value: val,
	})
	return m, nil
}

func (p *permission) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var m map[string]interface{}
	if err := unmarshal(&m); err != nil {
		return err
	}

	if err := p.fillType(m); err != nil {
		return err
	}

	return p.fillMethods(m)
}

func (p *permission) fillType(m map[string]interface{}) error {
	vh, ok1 := m[permHashKey]
	vg, ok2 := m[permGroupKey]
	switch {
	case ok1 && ok2:
		return errors.New("permission must have either 'hash' or 'group' field")
	case ok1:
		s, ok := vh.(string)
		if !ok {
			return errors.New("invalid 'hash' type")
		}

		u, err := util.Uint160DecodeStringLE(s)
		if err != nil {
			return err
		}

		p.Contract.Type = manifest.PermissionHash
		p.Contract.Value = u
	case ok2:
		s, ok := vg.(string)
		if !ok {
			return errors.New("invalid 'hash' type")
		}

		pub, err := keys.NewPublicKeyFromString(s)
		if err != nil {
			return err
		}

		p.Contract.Type = manifest.PermissionGroup
		p.Contract.Value = pub
	default:
		p.Contract.Type = manifest.PermissionWildcard
	}
	return nil
}

func (p *permission) fillMethods(m map[string]interface{}) error {
	methods, ok := m[permMethodKey]
	if !ok {
		return errors.New("'methods' field is missing from permission")
	}

	switch mt := methods.(type) {
	case string:
		if mt == "*" {
			p.Methods.Value = nil
			return nil
		}
	case []interface{}:
		ms := make([]string, len(mt))
		for i := range mt {
			ms[i], ok = mt[i].(string)
			if !ok {
				return errors.New("invalid permission method name")
			}
		}
		p.Methods.Value = ms
		return nil
	default:
	}
	return errors.New("'methods' field is invalid")
}
