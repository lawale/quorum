package model

import (
	"time"

	"github.com/google/uuid"
)

// Operator represents a console user who can manage policies, webhooks,
// and other administrative tasks through the Quorum admin console.
type Operator struct {
	ID                 uuid.UUID `json:"id"`
	Username           string    `json:"username"`
	PasswordHash       string    `json:"-"` // never expose in JSON
	DisplayName        string    `json:"display_name"`
	MustChangePassword bool      `json:"must_change_password"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
