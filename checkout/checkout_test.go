package checkout

import (
	"fmt"
	"math/rand"
	"time"
	"testing"
)

var hostname string = "localhost:7171"
var pubkey string = "032c14e1170737f3b1659c371f3e9f830bc308e1cc92b8a7cdd97918038dd45601"
var testlabel string = "cln-checkout-test"

func TestInit(t *testing.T) {
	// FIXME: startup a CLN node!
	err := Init(hostname, pubkey, testlabel)
	if err != nil {
		t.Fatalf("expected no err, got %s", err)	
	}
}

func TestInitEmpty(t *testing.T) {
	err := Init("", "", "")

	if err == nil {
		t.Fatalf("expected err, got none")
	}
}

func TestInvoiceWatchExpired(t *testing.T) {
	label := fmt.Sprintf("%s-%d", testlabel, rand.Uint32())
	err := Init(hostname, pubkey, label)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	/* Setup and register, get some messages out! */
	msgbus := make(chan *InvoiceEvent)
	err = RegisterForInvoiceUpdates(msgbus)
	if err != nil {
		t.Fatalf("err settingup invoice updates %s", err)
	}

	/* Start at lastindex = 0 */
	err = StartInvoiceWatch(0)
	if err != nil {
		t.Fatalf("err starting invoice watch %s", err)
	}

	inv, err := NewRestrictedInvoice(555, 1, "lets go!")		
	if err != nil {
		t.Fatalf("err new invoice %s", err)
	}


	/* Either we get an invoice timeout, or we timeout */
	exit := false
	for !exit {
		select {
		case invEvent := <- msgbus:
			if invEvent.Status == "expired" {
				exit = true
			}
			if invEvent.Label != inv.Label {
				t.Fatalf("event label not as expected? actual %s != expected %s. %v", invEvent.Label, inv.Label, invEvent)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out!")
		}
	}
}
