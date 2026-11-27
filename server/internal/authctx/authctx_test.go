package authctx

import (
	"context"
	"testing"
)

func TestAPIKeyID(t *testing.T) {
	if _, ok := APIKeyID(context.Background()); ok {
		t.Fatal("a plain context must carry no API key id")
	}
	id, ok := APIKeyID(WithAPIKeyID(context.Background(), 42))
	if !ok || id != 42 {
		t.Fatalf("APIKeyID = %d, %v; want 42, true", id, ok)
	}
}
