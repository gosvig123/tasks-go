package task

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewID returns a random 128-bit identifier in UUID text form.
func NewID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("generating task ID: %v", err))
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:])
}

// EnsureID assigns an identifier when the task does not have one.
func (t *Task) EnsureID() bool {
	if t.ID != "" {
		return false
	}
	t.ID = NewID()
	return true
}

// SameIdentity compares IDs when available and otherwise falls back to content.
func SameIdentity(left, right *Task) bool {
	if left == nil || right == nil {
		return false
	}
	if left.ID != "" && right.ID != "" {
		return left.ID == right.ID
	}
	return left.Content == right.Content
}
