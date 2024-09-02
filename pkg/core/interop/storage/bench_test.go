package storage_test

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func BenchmarkStorageFind(b *testing.B) {
	for count := 10; count <= 10000; count *= 10 {
		b.Run(fmt.Sprintf("%dElements", count), func(b *testing.B) {
			v, contractState, context, _ := createVMAndContractState(b)
			require.NoError(b, native.PutContractState(context.DAO, contractState))

			items := make(map[string]state.StorageItem)
			for range count {
				items["abc"+random.String(10)] = random.Bytes(10)
			}
			for k, v := range items {
				context.DAO.PutStorageItem(contractState.ID, []byte(k), v)
				context.DAO.PutStorageItem(contractState.ID+1, []byte(k), v)
			}
			changes, err := context.DAO.Persist()
			require.NoError(b, err)
			require.NotEqual(b, 0, changes)

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				b.StopTimer()
				v.Estack().PushVal(istorage.FindDefault)
				v.Estack().PushVal("abc")
				v.Estack().PushVal(stackitem.NewInterop(&istorage.Context{ID: contractState.ID}))
				b.StartTimer()
				err := istorage.Find(context)
				if err != nil {
					b.FailNow()
				}
				b.StopTimer()
				context.Finalize()
			}
		})
	}
}

func BenchmarkStorageFindIteratorNext(b *testing.B) {
	for count := 10; count <= 10000; count *= 10 {
		cases := map[string]int{
			"Pick1":    1,
			"PickHalf": count / 2,
			"PickAll":  count,
		}
		b.Run(fmt.Sprintf("%dElements", count), func(b *testing.B) {
			for name, last := range cases {
				b.Run(name, func(b *testing.B) {
					v, contractState, context, _ := createVMAndContractState(b)
					require.NoError(b, native.PutContractState(context.DAO, contractState))

					items := make(map[string]state.StorageItem)
					for range count {
						items["abc"+random.String(10)] = random.Bytes(10)
					}
					for k, v := range items {
						context.DAO.PutStorageItem(contractState.ID, []byte(k), v)
						context.DAO.PutStorageItem(contractState.ID+1, []byte(k), v)
					}
					changes, err := context.DAO.Persist()
					require.NoError(b, err)
					require.NotEqual(b, 0, changes)
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						b.StopTimer()
						v.Estack().PushVal(istorage.FindDefault)
						v.Estack().PushVal("abc")
						v.Estack().PushVal(stackitem.NewInterop(&istorage.Context{ID: contractState.ID}))
						b.StartTimer()
						err := istorage.Find(context)
						b.StopTimer()
						if err != nil {
							b.FailNow()
						}
						res := context.VM.Estack().Pop().Item()
						for range last {
							context.VM.Estack().PushVal(res)
							b.StartTimer()
							require.NoError(b, iterator.Next(context))
							b.StopTimer()
							require.True(b, context.VM.Estack().Pop().Bool())
						}

						context.VM.Estack().PushVal(res)
						require.NoError(b, iterator.Next(context))
						actual := context.VM.Estack().Pop().Bool()
						if last == count {
							require.False(b, actual)
						} else {
							require.True(b, actual)
						}
						context.Finalize()
					}
				})
			}
		})
	}
}
