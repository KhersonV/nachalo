package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gameservice/repository"
)

func TestWriteEquipmentErrorMapsExpectedValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "wrong class restriction",
			err:        repository.ErrEquipmentClassRestricted,
			wantStatus: http.StatusBadRequest,
			wantCode:   "class_restricted",
		},
		{
			name:       "item not inventory",
			err:        repository.ErrEquipmentItemNotInInventory,
			wantStatus: http.StatusBadRequest,
			wantCode:   "item_not_in_inventory",
		},
		{
			name:       "item not owned",
			err:        repository.ErrEquipmentItemNotOwned,
			wantStatus: http.StatusForbidden,
			wantCode:   "item_not_owned",
		},
		{
			name:       "character not owned",
			err:        repository.ErrEquipmentCharacterNotOwned,
			wantStatus: http.StatusForbidden,
			wantCode:   "character_not_owned",
		},
		{
			name:       "item not found",
			err:        repository.ErrEquipmentItemNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "item_not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeEquipmentError(rec, fmt.Errorf("wrapped validation: %w", tt.err))

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d body=%s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != tt.wantCode {
				t.Fatalf("expected error code %q, got %q", tt.wantCode, body["error"])
			}
		})
	}
}
