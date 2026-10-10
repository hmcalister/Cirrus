package email

import (
	"errors"
	"fmt"
	"net/textproto"
	"strings"
	"testing"
)

func TestRecipientErrorError(t *testing.T) {
	err := &RecipientError{Recipient: "bob@example.com", Err: ErrDelivery}

	msg := err.Error()
	if !strings.Contains(msg, "bob@example.com") {
		t.Errorf("error message = %q, want it to mention the recipient", msg)
	}
	if !strings.Contains(msg, ErrDelivery.Error()) {
		t.Errorf("error message = %q, want it to mention the cause", msg)
	}
}

func TestRecipientErrorUnwrap(t *testing.T) {
	err := &RecipientError{Recipient: "bob@example.com", Err: ErrDelivery}

	if !errors.Is(err, ErrDelivery) {
		t.Error("RecipientError should unwrap to ErrDelivery")
	}
	if errors.Is(err, ErrConnection) {
		t.Error("RecipientError should not match an unrelated sentinel")
	}
}

func TestRecipientErrorInJoin(t *testing.T) {
	bad := []error{
		&RecipientError{Recipient: "a@example.com", Err: ErrDelivery},
		&RecipientError{Recipient: "b@example.com", Err: ErrConnection},
	}
	joined := errors.Join(bad...)

	if !errors.Is(joined, ErrDelivery) {
		t.Error("joined error should match ErrDelivery")
	}
	if !errors.Is(joined, ErrConnection) {
		t.Error("joined error should match ErrConnection")
	}

	var recErr *RecipientError
	if !errors.As(joined, &recErr) {
		t.Fatal("joined error should contain a RecipientError")
	}
}

func TestWrapSMTPErrorDelivery(t *testing.T) {
	// A server response is a delivery failure.
	err := WrapSMTPError(&textproto.Error{Code: 550, Msg: "mailbox unavailable"})

	if !errors.Is(err, ErrDelivery) {
		t.Errorf("err = %v, want ErrDelivery", err)
	}
	if errors.Is(err, ErrConnection) {
		t.Error("server response should not be classified as a connection error")
	}
}

func TestWrapSMTPErrorConnection(t *testing.T) {
	// A non-protocol error is a connection failure.
	err := WrapSMTPError(fmt.Errorf("broken pipe"))

	if !errors.Is(err, ErrConnection) {
		t.Errorf("err = %v, want ErrConnection", err)
	}
	if errors.Is(err, ErrDelivery) {
		t.Error("non-protocol error should not be classified as a delivery error")
	}
}

func TestWrapSMTPErrorPreservesCause(t *testing.T) {
	original := &textproto.Error{Code: 550, Msg: "mailbox unavailable"}
	err := WrapSMTPError(original)

	var smtpErr *textproto.Error
	if !errors.As(err, &smtpErr) {
		t.Fatal("wrapped error should still be discoverable as *textproto.Error")
	}
	if smtpErr.Code != 550 {
		t.Errorf("code = %d, want 550", smtpErr.Code)
	}
}
