package service

import (
	"context"
	"errors"
	"testing"
)

func TestQueryContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := queryContextError(ctx); !errors.Is(err, ErrQueryCanceled) {
		t.Fatalf("canceled context error = %v, want ErrQueryCanceled", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if err := queryContextError(ctx); err != nil {
		t.Fatalf("active context error = %v, want nil", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 0)
	defer cancel()
	if err := queryContextError(ctx); !errors.Is(err, ErrQueryTimeout) {
		t.Fatalf("deadline context error = %v, want ErrQueryTimeout", err)
	}
}
