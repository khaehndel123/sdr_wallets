package notifier

import (
	"context"

	"backend/app/models"
)

type Service interface {
	Subscribe(ctx context.Context, subscription *models.NewSubscription) error
	Notify(ctx context.Context, notification *models.Notification)
}
