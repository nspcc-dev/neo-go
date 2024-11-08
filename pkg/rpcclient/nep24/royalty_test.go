package nep24

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type testAct struct {
	err error
	res *result.Invoke
}

func (t *testAct) Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error) {
	return t.res, t.err
}

func TestRoyaltyReaderRoyaltyInfo(t *testing.T) {
	ta := new(testAct)
	rr := NewRoyaltyReader(ta, util.Uint160{1, 2, 3})

	tokenID := []byte{1, 2, 3}
	royaltyToken := util.Uint160{4, 5, 6}
	salePrice := big.NewInt(1000)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make([]stackitem.Item{
					stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
					stackitem.Make(big.NewInt(100)),
				}),
				stackitem.Make([]stackitem.Item{
					stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
					stackitem.Make(big.NewInt(200)),
				}),
			}),
		},
	}
	ri, err := rr.RoyaltyInfo(tokenID, royaltyToken, salePrice)
	require.NoError(t, err)
	require.Equal(t, []RoyaltyRecipient{
		{
			Address: util.Uint160{7, 8, 9},
			Amount:  big.NewInt(100),
		},
		{
			Address: util.Uint160{7, 8, 9},
			Amount:  big.NewInt(200),
		},
	}, ri)

	ta.err = errors.New("")
	_, err = rr.RoyaltyInfo(tokenID, royaltyToken, salePrice)
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
			}),
		},
	}
	_, err = rr.RoyaltyInfo(tokenID, royaltyToken, salePrice)
	require.Error(t, err)
}

func TestRoyaltyRecipient_FromStackItem(t *testing.T) {
	tests := map[string]struct {
		items    []stackitem.Item
		err      error
		expected RoyaltyRecipient
	}{
		"good": {
			items: []stackitem.Item{
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
				stackitem.Make(big.NewInt(100)),
			},
			err: nil,
			expected: RoyaltyRecipient{
				Address: util.Uint160{7, 8, 9},
				Amount:  big.NewInt(100),
			},
		},
		"invalid number of items": {
			items: []stackitem.Item{
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
			},
			err: fmt.Errorf("invalid royalty structure: expected 2 items, got 1"),
		},
		"invalid recipient size": {
			items: []stackitem.Item{
				stackitem.Make([]byte{1, 2}),
				stackitem.Make(big.NewInt(100)),
			},
			err: fmt.Errorf("invalid recipient address: expected byte size of 20 got 2"),
		},
		"invalid recipient type": {
			items: []stackitem.Item{
				stackitem.Make([]int{7, 8, 9}),
				stackitem.Make(big.NewInt(100)),
			},
			err: fmt.Errorf("failed to decode recipient address: invalid conversion: Array/ByteString"),
		},
		"invalid amount type": {
			items: []stackitem.Item{
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
				stackitem.Make([]int{7, 8, 9}),
			},
			err: fmt.Errorf("failed to decode royalty amount: invalid conversion: Array/Integer"),
		},
		"negative amount": {
			items: []stackitem.Item{
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
				stackitem.Make(big.NewInt(-100)),
			},
			err: fmt.Errorf("negative royalty amount"),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var ri RoyaltyRecipient
			err := ri.FromStackItem(tt.items)
			if tt.err != nil {
				require.EqualError(t, err, tt.err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, ri)
			}
		})
	}
}

func TestRoyaltiesTransferredEventFromStackitem(t *testing.T) {
	tests := []struct {
		name      string
		item      *stackitem.Array
		expectErr bool
		expected  *RoyaltiesTransferredEvent
	}{
		{
			name: "good",
			item: stackitem.NewArray([]stackitem.Item{
				stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()), // RoyaltyToken
				stackitem.Make(util.Uint160{4, 5, 6}.BytesBE()), // RoyaltyRecipient
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()), // Buyer
				stackitem.Make([]byte{1, 2, 3}),                 // TokenID
				stackitem.Make(big.NewInt(100)),                 // Amount
			}),
			expectErr: false,
			expected: &RoyaltiesTransferredEvent{
				RoyaltyToken:     util.Uint160{1, 2, 3},
				RoyaltyRecipient: util.Uint160{4, 5, 6},
				Buyer:            util.Uint160{7, 8, 9},
				TokenID:          []byte{1, 2, 3},
				Amount:           big.NewInt(100),
			},
		},
		{
			name: "invalid number of items",
			item: stackitem.NewArray([]stackitem.Item{
				stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()), // Only one item
			}),
			expectErr: true,
		},
		{
			name: "invalid recipient size",
			item: stackitem.NewArray([]stackitem.Item{
				stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()), // RoyaltyToken
				stackitem.Make([]byte{1, 2}),                    // Invalid RoyaltyRecipient
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()), // Buyer
				stackitem.Make([]byte{1, 2, 3}),                 // TokenID
				stackitem.Make(big.NewInt(100)),                 // Amount
			}),
			expectErr: true,
		},
		{
			name: "invalid integer amount",
			item: stackitem.NewArray([]stackitem.Item{
				stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()), // RoyaltyToken
				stackitem.Make(util.Uint160{4, 5, 6}.BytesBE()), // RoyaltyRecipient
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()), // Buyer
				stackitem.Make([]byte{1, 2, 3}),                 // TokenID
				stackitem.Make(stackitem.NewStruct(nil)),        // Invalid integer for Amount
			}),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := new(RoyaltiesTransferredEvent)
			err := event.FromStackItem(tt.item)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, event)
			}
		})
	}
}

func TestRoyaltiesTransferredEventsFromApplicationLog(t *testing.T) {
	createEvent := func(token, recipient, buyer util.Uint160, tokenID []byte, amount *big.Int) state.NotificationEvent {
		return state.NotificationEvent{
			ScriptHash: util.Uint160{1, 2, 3}, // Any contract address.
			Name:       "RoyaltiesTransferred",
			Item: stackitem.NewArray([]stackitem.Item{
				stackitem.Make(token.BytesBE()),     // RoyaltyToken
				stackitem.Make(recipient.BytesBE()), // RoyaltyRecipient
				stackitem.Make(buyer.BytesBE()),     // Buyer
				stackitem.Make(tokenID),             // TokenID
				stackitem.Make(amount),              // Amount
			}),
		}
	}

	tests := []struct {
		name      string
		log       *result.ApplicationLog
		expectErr bool
		expected  []*RoyaltiesTransferredEvent
	}{
		{
			name: "valid log with one event",
			log: &result.ApplicationLog{
				Executions: []state.Execution{
					{
						Events: []state.NotificationEvent{
							createEvent(
								util.Uint160{1, 2, 3}, // RoyaltyToken
								util.Uint160{4, 5, 6}, // RoyaltyRecipient
								util.Uint160{7, 8, 9}, // Buyer
								[]byte{1, 2, 3},       // TokenID
								big.NewInt(100),       // Amount
							),
						},
					},
				},
			},
			expectErr: false,
			expected: []*RoyaltiesTransferredEvent{
				{
					RoyaltyToken:     util.Uint160{1, 2, 3},
					RoyaltyRecipient: util.Uint160{4, 5, 6},
					Buyer:            util.Uint160{7, 8, 9},
					TokenID:          []byte{1, 2, 3},
					Amount:           big.NewInt(100),
				},
			},
		},
		{
			name: "invalid event structure (missing fields)",
			log: &result.ApplicationLog{
				Executions: []state.Execution{
					{
						Events: []state.NotificationEvent{
							{
								Name: "RoyaltiesTransferred",
								Item: stackitem.NewArray([]stackitem.Item{
									stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()), // RoyaltyToken
									// Missing other fields
								}),
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name:      "empty log",
			log:       &result.ApplicationLog{},
			expectErr: false,
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := RoyaltiesTransferredEventsFromApplicationLog(tt.log)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, events)
			}
		})
	}
}
