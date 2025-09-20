package billing

import (
	"testing"

	"github.com/google/uuid"
)

func TestCreditLock(t *testing.T) {
	lock := &CreditLock{
		ID:        uuid.New(),
		ClientID:  uuid.New(),
		MessageID: uuid.New(),
		Amount:    100,
		State:     "HELD",
	}

	if lock.State != "HELD" {
		t.Errorf("Expected state HELD, got %s", lock.State)
	}

	if lock.Amount != 100 {
		t.Errorf("Expected amount 100, got %d", lock.Amount)
	}
}
