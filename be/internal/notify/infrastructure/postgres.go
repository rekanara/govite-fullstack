// Package infrastructure stores notifications in Postgres and adapts FCM.
package infrastructure

import (
	"context"
	"encoding/json"

	"govite/internal/kernel"
	"govite/internal/notify/domain"
	"govite/internal/platform/fcm"
	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
)

type Repository struct{ DB *postgres.DB }

func toNotification(n store.Notification) domain.Notification {
	return domain.Notification{ID: n.ID, Type: n.Type, Title: n.Title, Body: n.Body, Data: n.Data, ReadAt: n.ReadAt, CreatedAt: n.CreatedAt}
}

func (r Repository) ActiveUserIDs(ctx context.Context, after string, limit int32) ([]string, error) {
	return r.DB.Q(ctx).ActiveUserIDs(ctx, store.ActiveUserIDsParams{After: after, Lim: limit})
}

func (r Repository) Insert(ctx context.Context, in domain.Input, data json.RawMessage, ids, userIDs []string) ([]string, error) {
	return r.DB.Q(ctx).InsertNotifications(ctx, store.InsertNotificationsParams{Type: in.Type, Title: in.Title, Body: in.Body, Data: data, DedupeKey: in.DedupeKey, Ids: ids, UserIds: userIDs})
}

func (r Repository) Devices(ctx context.Context, userIDs []string, after string, limit int32) ([]domain.Device, error) {
	rows, err := r.DB.Q(ctx).PushDevices(ctx, store.PushDevicesParams{UserIds: userIDs, After: after, Lim: limit})
	out := make([]domain.Device, len(rows))
	for i, d := range rows {
		out[i] = domain.Device{ID: d.ID, Token: d.Token}
	}
	return out, err
}

func (r Repository) DeleteDeviceTokens(ctx context.Context, tokens []string) error {
	return r.DB.Q(ctx).DeleteDevicesByToken(ctx, tokens)
}

func (r Repository) RegisterDevice(ctx context.Context, userID, deviceID, token string, device json.RawMessage) error {
	q := r.DB.Q(ctx)
	// A token belongs to one device: drop it from any other owner first.
	if err := q.DeleteDeviceTokenElsewhere(ctx, store.DeleteDeviceTokenElsewhereParams{Token: token, UserID: userID, DeviceID: deviceID}); err != nil {
		return err
	}
	return q.UpsertDevice(ctx, store.UpsertDeviceParams{ID: kernel.NewID(), UserID: userID, DeviceID: deviceID, Token: token, Device: device})
}

func (r Repository) DeleteDevice(ctx context.Context, userID, deviceID string) error {
	return r.DB.Q(ctx).DeleteDevice(ctx, store.DeleteDeviceParams{UserID: userID, DeviceID: deviceID})
}

func (r Repository) Counts(ctx context.Context, userID string) (int64, int64, error) {
	c, err := r.DB.Q(ctx).CountNotifications(ctx, userID)
	return c.Total, c.Unread, err
}

func (r Repository) List(ctx context.Context, userID string, unreadOnly bool, limit, offset int32) ([]domain.Notification, error) {
	rows, err := r.DB.Q(ctx).ListNotifications(ctx, store.ListNotificationsParams{UserID: userID, UnreadOnly: unreadOnly, Lim: limit, Off: offset})
	out := make([]domain.Notification, len(rows))
	for i, n := range rows {
		out[i] = toNotification(n)
	}
	return out, err
}

func (r Repository) MarkAllRead(ctx context.Context, userID string) error {
	return r.DB.Q(ctx).MarkAllNotificationsRead(ctx, userID)
}

func (r Repository) MarkRead(ctx context.Context, id, userID string) (domain.Notification, error) {
	n, err := r.DB.Q(ctx).MarkNotificationRead(ctx, store.MarkNotificationReadParams{ID: id, UserID: userID})
	return toNotification(n), err
}

// Pusher adapts an fcm.Pusher to domain.Pusher.
type Pusher struct{ fcm.Pusher }

func (p Pusher) Send(ctx context.Context, tokens []string, m domain.PushMessage) []string {
	return p.Pusher.Send(ctx, tokens, fcm.PushMessage{Title: m.Title, Body: m.Body, ImageURL: m.ImageURL, Data: m.Data})
}
