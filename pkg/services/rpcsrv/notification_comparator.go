package rpcsrv

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/rpcevent"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
)

// notificationEventComparator is a comparator for notification events.
type notificationEventComparator struct {
	filter neorpc.SubscriptionFilter
}

// EventID returns the event ID for the notification event comparator.
func (s notificationEventComparator) EventID() neorpc.EventID {
	return neorpc.NotificationEventID
}

// Filter returns the filter for the notification event comparator.
func (c notificationEventComparator) Filter() neorpc.SubscriptionFilter {
	return c.filter
}

// notificationEventContainer is a container for a notification event.
type notificationEventContainer struct {
	ntf *state.ContainedNotificationEvent
}

// EventID returns the event ID for the notification event container.
func (c notificationEventContainer) EventID() neorpc.EventID {
	return neorpc.NotificationEventID
}

// EventPayload returns the payload for the notification event container.
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
		if filter == nil || rpcevent.Matches(&notificationEventComparator{
			filter: *filter,
		}, &notificationEventContainer{ntf: &ntf}) {
			notifications = append(notifications, ntf)
		}
	}
	return notifications
}
