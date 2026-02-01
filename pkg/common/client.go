package common

import (
	"context"

	"cloud.google.com/go/firestore"
)

func NewFirestoreClient(ctx context.Context, projectID string) (*firestore.Client, error) {
	// If FIRESTORE_EMULATOR_HOST is set, the client library automatically uses it.
	// We just need to make sure projectID matches.
	return firestore.NewClient(ctx, projectID)
}
