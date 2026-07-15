package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestComponentError(t *testing.T) {
	testErr := errors.New("component failed")

	tests := []struct {
		name      string
		canceled  bool
		resultErr error
		wantErr   bool
	}{
		{name: "nil while running", wantErr: true},
		{name: "canceled while running", resultErr: context.Canceled, wantErr: true},
		{name: "server closed while running", resultErr: http.ErrServerClosed, wantErr: true},
		{name: "failure while running", resultErr: testErr, wantErr: true},
		{name: "nil after cancellation", canceled: true},
		{name: "canceled after cancellation", canceled: true, resultErr: context.Canceled},
		{name: "server closed after cancellation", canceled: true, resultErr: http.ErrServerClosed},
		{name: "failure after cancellation", canceled: true, resultErr: testErr, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if tt.canceled {
				cancel()
			} else {
				defer cancel()
			}

			err := componentError(componentResult{name: "test component", err: tt.resultErr}, ctx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("componentError() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.resultErr != nil && tt.wantErr && !errors.Is(err, tt.resultErr) {
				t.Fatalf("componentError() error = %v, want wrapped error %v", err, tt.resultErr)
			}
		})
	}
}
