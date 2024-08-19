package nep11

import (
	"errors"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestRoyaltyReaderRoyaltyInfo(t *testing.T) {
	ta := new(testAct)
	rr := NewRoyaltyReader(ta, util.Uint160{1, 2, 3})

	tokenID := []byte{1, 2, 3}
	royaltyToken := util.Uint160{4, 5, 6}
	salePrice := big.NewInt(1000)

	tests := []struct {
		name       string
		setupFunc  func()
		expectErr  bool
		expectedRI []RoyaltyInfoDetail
	}{
		{
			name: "error case",
			setupFunc: func() {
				ta.err = errors.New("some error")
			},
			expectErr: true,
		},
		{
			name: "valid response",
			setupFunc: func() {
				ta.err = nil
				recipient := util.Uint160{7, 8, 9}
				amount := big.NewInt(100)
				ta.res = &result.Invoke{
					State: "HALT",
					Stack: []stackitem.Item{stackitem.Make([]stackitem.Item{
						stackitem.Make(recipient.BytesBE()),
						stackitem.Make(amount),
					})},
				}
			},
			expectErr: false,
			expectedRI: []RoyaltyInfoDetail{
				{RoyaltyRecipient: util.Uint160{7, 8, 9}, RoyaltyAmount: big.NewInt(100)},
			},
		},
		{
			name: "invalid data response",
			setupFunc: func() {
				ta.res = &result.Invoke{
					State: "HALT",
					Stack: []stackitem.Item{
						stackitem.Make([]stackitem.Item{
							stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
						}),
					},
				}
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFunc()
			ri, err := rr.RoyaltyInfo(tokenID, royaltyToken, salePrice)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedRI, ri)
			}
		})
	}
}

func TestItemToRoyaltyInfoDetail(t *testing.T) {
	tests := []struct {
		name      string
		items     []stackitem.Item
		expectErr bool
		expected  *RoyaltyInfoDetail
	}{
		{
			name: "valid input",
			items: []stackitem.Item{
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
				stackitem.Make(big.NewInt(100)),
			},
			expectErr: false,
			expected: &RoyaltyInfoDetail{
				RoyaltyRecipient: util.Uint160{7, 8, 9},
				RoyaltyAmount:    big.NewInt(100),
			},
		},
		{
			name: "invalid number of items",
			items: []stackitem.Item{
				stackitem.Make(util.Uint160{7, 8, 9}.BytesBE()),
			},
			expectErr: true,
		},
		{
			name: "invalid recipient size",
			items: []stackitem.Item{
				stackitem.Make([]byte{1, 2}),
				stackitem.Make(big.NewInt(100)),
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ri, err := itemToRoyaltyInfoDetail(tt.items)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, ri)
			}
		})
	}
}

func TestFromStackItem(t *testing.T) {
	tests := []struct {
		name      string
		item      *stackitem.Array
		expectErr bool
		expected  *RoyaltiesTransferredEvent
	}{
		{
			name: "valid stack item",
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
				stackitem.Make(stackitem.NewBool(true)),         // Invalid integer for Amount
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
