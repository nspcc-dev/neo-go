/*
Package core implements Neo ledger functionality.
It's built around the Blockchain structure that maintains state of the ledger.

# Events

You can subscribe to Blockchain events using a set of Subscribe and Unsubscribe
methods. These methods accept channels that will be used to send appropriate
events, so you can control buffering. Channels are never closed by Blockchain,
you can close them after unsubscription.

Unlike RPC-level subscriptions these don't allow event filtering because it
doesn't improve overall efficiency much (when you're using Blockchain you're
in the same process with it and filtering on your side is not that different
from filtering on Blockchain side).

The same level of ordering guarantees as with RPC subscriptions is provided,
albeit for a set of event channels, so at first transaction execution is
announced via appropriate channels, then followed by notifications generated
during this execution, then followed by transaction announcement and then
followed by block announcement. Transaction announcements are ordered the same
way they're stored in the block.

Be careful using these subscriptions, this mechanism is not intended to be used
by lots of subscribers and failing to read from event channels can affect
other Blockchain operations.
*/
package core
