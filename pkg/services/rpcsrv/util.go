package rpcsrv

import (
	"errors"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/rpcevent"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
)

func checkUint32(i int) error {
	if i < 0 || i > math.MaxUint32 {
		return errors.New("value should fit uint32")
	}
	return nil
}

func checkInt32(i int) error {
	if i < math.MinInt32 || i > math.MaxInt32 {
		return errors.New("value should fit int32")
	}
	return nil
}

func (c notificationComparatorFilter) EventID() neorpc.EventID {
	return c.id
}

func (c notificationComparatorFilter) Filter() neorpc.SubscriptionFilter {
	return c.filter
}

func (c notificationEventContainer) EventID() neorpc.EventID {
	return neorpc.NotificationEventID
}

func (c notificationEventContainer) EventPayload() any {
	return c.ntf
}

func processAppExecResults(aers []state.AppExecResult, filter *neorpc.NotificationFilter) []state.ContainedNotificationEvent {
	var notifications []state.ContainedNotificationEvent
	for _, aer := range aers {
		if aer.VMState == vmstate.Halt {
			notifications = append(notifications, filterEvents(aer.Events, aer.Container, filter)...)
		}
	}
	return notifications
}

func filterEvents(events []state.NotificationEvent, container util.Uint256, filter *neorpc.NotificationFilter) []state.ContainedNotificationEvent {
	var notifications []state.ContainedNotificationEvent
	for _, evt := range events {
		ntf := state.ContainedNotificationEvent{
			Container:         container,
			NotificationEvent: evt,
		}
		if filter == nil || rpcevent.Matches(&notificationComparatorFilter{
			id:     neorpc.NotificationEventID,
			filter: *filter,
		}, &notificationEventContainer{ntf: &ntf}) {
			notifications = append(notifications, ntf)
		}
	}
	return notifications
}
