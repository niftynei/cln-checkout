package checkout


import (
	"fmt"
	"strings"
	"time"
	"github.com/base58btc/clnsocket"
)

/* FIXME: make this an envvar */
var token string = "6qthVaDryzXm-NpJXdJ4mReZ06auzaMWSQ0pY6HnX8Q9MA==";

/* Ok let's go.
 * - what happens if lnsocket connection drops?
 * - fixme: handle reconnects/retries to lightning node
 * - fixme: event bus for disconnect/reconnect events 
 *   (so client knows to turn on/off btc checkout)
 */

type (
	InvoiceEvent struct {
		Label string 	
		Invstring string
		Status string
		AmountRequested uint64
		AmountReceived uint64
		Description string
		ExpiresAt time.Time
		PaidAt *time.Time
		PayHash string
		ProofOfPayment string
		UpdateIndex uint64
	}
)

/* Internal state for checkout module */
var subs []chan *InvoiceEvent
var labelsTag string
var runInvoice bool
var invoiceDone chan bool

func initState(labelTag string) error {
	if len(subs) > 0 {
		return fmt.Errorf("Already called? subscriptions exist")
	}
	subs = make([]chan *InvoiceEvent, 0)

	labelsTag = labelTag

	invoiceDone = make(chan bool)

	return nil
}

func Init(hostname, pubkey, labelTag string) error {
	err := clnsocket.Init(hostname, pubkey)
	if err != nil {
		return err
	}

	/* We require the new unified invoices feature */
	onchainInvoices, err := clnsocket.HasOnchainInvoices(token)
	if err != nil {
		return err
	}

	if !onchainInvoices {
		return fmt.Errorf("We require onchain invoices turned on. Set `invoices-onchain-fallback` in your lightning config")
	}

	return initState(labelTag)
}

func StartInvoiceWatch(indexStart uint64) error {
	if runInvoice {
		return fmt.Errorf("Invoice poll already running")
	}
	runInvoice = true
	/* FIXME: keep track of this so we don't have more than one? 
	 * and can keep track of state */
	// what to do about errors?
	go runInvoices(indexStart)
	return nil
}

func StopInvoiceWatch() error {
	runInvoice = false
	return nil
}

func labelMatch(label, labelTag string) bool {
	return strings.HasPrefix(label, labelTag)
}

func filterInvoices(invoices []*clnsocket.Invoice) []*InvoiceEvent {
	filtered := make([]*InvoiceEvent, 0)

	for _, invoice := range invoices {
		if labelMatch(invoice.Label, labelsTag) {
			filtered = append(filtered, toInvoiceEvent(invoice))
		}
	}

	return filtered
}

func toInvoiceEvent(invoice *clnsocket.Invoice) *InvoiceEvent {
	return &InvoiceEvent {
		Label: invoice.Label,
		Invstring: invoice.Invstring,
		Status: invoice.Status,
		AmountRequested: invoice.Amount,
		AmountReceived: invoice.AmountReceived,
		Description: invoice.Description,
		ExpiresAt: invoice.ExpiresAt,
		PaidAt: invoice.PaidAt,
		PayHash: invoice.PayHash,
		ProofOfPayment: invoice.ProofOfPayment,
		UpdateIndex: invoice.UpdatedIndex,
	}
}

func runInvoices(startIndex uint64) error {
	/* Poll invoice until we get some back! */
	index := startIndex
	for runInvoice {
		invoices, err := clnsocket.PollInvoices(token, index)

		if err != nil {
			runInvoice = false
			return err
		}

		/* Filter to only relevant invoices */
		filtered := filterInvoices(invoices)

		for _, inv := range filtered {
			for _, sub := range subs {
				sub <- inv
				/* FIXME: remove channels if/when closed? */
			}
		}

		/* Update last index */
		if len(invoices) > 0 {
			index = invoices[len(invoices) - 1].UpdatedIndex + 1
		}

		/* FIXME: make configurable */
		time.Sleep(500)
	}
	
	invoiceDone <- true
	return nil
}

func RegisterForInvoiceUpdates(msgchan chan *InvoiceEvent) error {
	if subs == nil {
		return fmt.Errorf("Not initialized! call Init first")
	}

	subs = append(subs, msgchan)

	return nil
}

func NewRestrictedInvoice(amt uint64, expiry uint64, desc string) (*clnsocket.InvoiceReq, error) {
	inv, err := clnsocket.NewInvoice(token, labelsTag, amt, expiry, desc)
	if err != nil {
		return nil, err
	}

	inv.WaitRune, err = clnsocket.RestrictToWaitInvoice(token, inv.Label)
	if err != nil {
		return nil, err
	}

	return inv, nil
}

func Shutdown() {
	StopInvoiceWatch()

	<- invoiceDone
	clnsocket.Shutdown()
}
