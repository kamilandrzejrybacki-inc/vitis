package model

import "time"

type Turn struct {
	SessionID string    `json:"session_id"`
	Index     int       `json:"turn_index"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
