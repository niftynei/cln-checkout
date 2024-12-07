package clnsocket

import (
	"testing"
	"time"
)

var token string = "6qthVaDryzXm-NpJXdJ4mReZ06auzaMWSQ0pY6HnX8Q9MA==";

func connectCLN() error {
	// FIXME: startup a CLN node!
	err := Init("localhost:7171", "032c14e1170737f3b1659c371f3e9f830bc308e1cc92b8a7cdd97918038dd45601")
	return err
}

func connectOrErr(t *testing.T) {
	err := connectCLN()
	if err != nil {
		t.Fatalf("expected no err, got %s", err)	
	}
}

func TestInit(t *testing.T) {
	connectOrErr(t)
}

func TestInitEmpty(t *testing.T) {
	err := Init("", "")

	if err == nil {
		t.Fatalf("expected err, got none")
	}
}

func TestOnchainInvoices(t *testing.T) {
	connectOrErr(t)

	_, err := HasOnchainInvoices(token)
	if err != nil {
		t.Fatalf("expected ok, failed: %s", err)
	}
}

func TestNewInvoice(t *testing.T) {
	connectOrErr(t)

	_, err := NewInvoice(token, "", 1600, 360, "new invoice!")
	if err != nil {
		t.Fatalf("expected ok, failed: %s", err)
	}
}

func TestRestrictedRune(t *testing.T) {
	connectOrErr(t)

	inv, err := NewInvoice(token, "", 1888, 1, "next invoice!")
	if err != nil {
		t.Fatalf("expected ok, failed: %s", err)
	}

	newToken, err := RestrictToWaitInvoice(token, inv.Label)
	if err != nil {
		t.Fatalf("expected ok, failed: %s", err)
	}
	
	/* Now try to get a new invoice with new Rune! */
	_, err = NewInvoice(newToken, "", 1999, 300, "not allowed")
	if err == nil {
		t.Fatalf("expected failure, instead was ok. offending rune %s", newToken)
	}

	status, err := WaitInvoice(newToken, inv.Label)
	if err != nil {
		t.Fatalf("wait invoice failed %s", err)
	}

	if status != "expired" {
		t.Fatalf("wait invoice invalid status. expected 'expired', got %s", status)
	}
}

func TestPollInvoices(t *testing.T) {
	connectOrErr(t)

	/* FIXME: use fresh CLN each test?
	 * for now, we zoom ahead to most recent/updated index */
	lastIndex := uint64(0)
	invs, err := PollInvoices(token, lastIndex)
	if err != nil {
		t.Fatalf("listinvoices failed %s", err)
	}
	
	if len(invs) > 0 {
		inv := invs[len(invs) - 1]
		/* Go one past the last! */
		lastIndex = inv.UpdatedIndex + 1
		clear(invs)
		invs = invs[:0]
	}

	if len(invs) != 0 {
		t.Fatalf("invs is not clear?")
	}
	
	invoice, err := NewInvoice(token, "", 1888, 1, "expiring invoice!")
	if err != nil {
		t.Fatalf("expected ok, failed: %s", err)
	}

	now := time.Now().Unix()
	for len(invs) == 0 && now + 5 > time.Now().Unix() {
		invs, err = PollInvoices(token, lastIndex)
		if err != nil {
			t.Fatalf("listinvoices failed %s", err)
		}
		time.Sleep(1 * time.Second)
	}
	
	if len(invs) == 0 {
		t.Fatalf("listinvoices poll didn't time out new invoice")
	}

	inv := invs[len(invs) - 1]
	if invoice.Label != inv.Label {
		t.Fatalf("listinvoices returned %v", inv)
	}
}
