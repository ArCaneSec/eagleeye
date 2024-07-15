package server

import (
	"context"
	"time"
)

func queryContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 4 * time.Second)
	return ctx
}