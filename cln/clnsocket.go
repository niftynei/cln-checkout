package clnsocket

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/niftynei/lnsocket/go"
	"github.com/tidwall/gjson"
)

var lightningConn lnsocket.LNSocket;

type (
	InvoiceReq struct {
		Invstring string
		WaitRune  string
		Expiry    time.Time
		PayHash   string
		Label     string
	}

	/* FIXME: auto gen this? */
	Invoice struct {
		Label string
		Invstring string
		PayHash string
		Status string
		Amount  uint64
		AmountReceived uint64
		Description string
		ExpiresAt time.Time
		CreatedIndex uint64
		UpdatedIndex uint64
		PayIndex uint64
		PaidAt *time.Time
		ProofOfPayment string
	}
)

func (i *Invoice) UnmarshalJSON(data []byte) error {

	var networkinv struct {
		Label string `json:"label"`
		Bolt11 string `json:"bolt11"`
		Bolt12 string`json:"bolt12"`
		PayHash string `json:"payment_hash"`
		Status string `json:"status"`
		Amount  uint64 `json:"amount_msat"`
		AmountReceived uint64 `json:"amount_received_msat"`
		Description string `json:"description"`
		ExpiresAt int64 `json:"expires_at"`
		CreatedIndex uint64 `json:"created_index"`
		UpdatedIndex uint64 `json:"updated_index"`
		PayIndex uint64 `json:"pay_index"`
		PaidAt int64 `json:"paid_at"`
		ProofOfPayment string `json:"payment_preimage"`
		LocalOfferID string  `json:"local_offer_id"`
		PayerNote string  `json:"invreq_payer_note"`
	}

	if err := json.Unmarshal(data, &networkinv); err != nil {
		return err
	}


	*i = Invoice{
		Label: networkinv.Label,
		PayHash: networkinv.PayHash,
		Status: networkinv.Status,
		Amount: networkinv.Amount,
		AmountReceived: networkinv.AmountReceived,
		Description: networkinv.Description,
		ExpiresAt: time.Unix(networkinv.ExpiresAt, 0),
		CreatedIndex: networkinv.CreatedIndex,
		UpdatedIndex: networkinv.UpdatedIndex,
		PayIndex: networkinv.PayIndex,
		ProofOfPayment: networkinv.ProofOfPayment,
	}

	if networkinv.PaidAt > 0 {
		paidAt := time.Unix(networkinv.PaidAt, 0)
		i.PaidAt = &paidAt
	}

	if networkinv.Bolt11 != "" {
		i.Invstring = networkinv.Bolt11
	}
	if networkinv.Bolt12 != "" {
		i.Invstring = networkinv.Bolt12
	}

	return nil
}

func setupLN() lnsocket.LNSocket {
	ln := lnsocket.LNSocket{}
	ln.GenKey()
	return ln
}

func Init(hostname, pubkey string) error {
	if hostname == "" || pubkey == "" {
		return fmt.Errorf("need both hostname + pubkey")
	}
	ln := setupLN()
	
	err := ln.Connect(hostname, pubkey)
	if err != nil {
		return err
	}

	err = ln.PerformInit()
	if err != nil {
		return err
	}

	lightningConn = ln
	rand.Seed(time.Now().Unix())
	return nil
}

func Shutdown() {
	lightningConn.Disconnect()
}

func HasOnchainInvoices(token string) (bool, error) {
	result, err := lightningConn.Rpc(token, "listconfigs", "[\"invoices-onchain-fallback\"]")
	if err != nil {
		return false, err
	}

	value := gjson.Get(result, "result.configs.invoices-onchain-fallback.set")
	if !value.Exists() {
		return false, fmt.Errorf("Unable to get config 'invoices-onchain-fallback'. %s", result)
	}
	return value.Bool(), nil	
}

func genLabel(labelTag string) string {
	if labelTag == "" {
		labelTag = "cln"
	}
	return fmt.Sprintf("%s-%d-%d", labelTag, rand.Uint32(), time.Now().UnixNano())
}

func NewInvoice(token string, labelTag string, amt uint64, expiry uint64, desc string) (*InvoiceReq, error) {
	label := genLabel(labelTag)
	result, err := lightningConn.Rpc(token, "invoice", 
		fmt.Sprintf("{\"amount_msat\":\"%dmsat\",\"label\":\"%s\",\"description\":\"%s\",\"expiry\":%d}", amt, label, desc, expiry))
	if err != nil {
		return nil, err
	}

	expiresAt := gjson.Get(result, "result.expires_at")
	bolt11 := gjson.Get(result, "result.bolt11")
	payHash := gjson.Get(result, "result.payment_hash")

	if !expiresAt.Exists() || !bolt11.Exists() || !payHash.Exists() {
		return nil, fmt.Errorf("Error parsing invoice result. %s", result)
	}

	return &InvoiceReq{
		Invstring: bolt11.String(),
		Expiry: time.Unix(expiresAt.Int(), 0),
		PayHash: payHash.String(),
		Label: label,
	}, nil
}

/* FIXME: use independent rune library? */
func RestrictToWaitInvoice(origToken string, label string) (string, error)  {
	restrictions := fmt.Sprintf("[[\"method=waitinvoice\"],[\"pnum=1\"],[\"parr0=%s\"]]", label)
	result, err := lightningConn.Rpc(origToken, "createrune",
		fmt.Sprintf("{\"rune\":\"%s\", \"restrictions\":%s}", origToken, restrictions))

	if err != nil {
		return "", err
	}

	restrictedToken := gjson.Get(result, "result.rune")
	if !restrictedToken.Exists() {
		return "", fmt.Errorf("rune failed to restrict. %s", result)
	}

	return restrictedToken.String(), nil
}

func WaitInvoice(token, label string) (string, error) {
	result, err := lightningConn.Rpc(token, "waitinvoice", fmt.Sprintf("[\"%s\"]", label))

	if err != nil {
		return "", err
	}
	status := gjson.Get(result, "result.status")
	if !status.Exists() {
		status = gjson.Get(result, "error.data.status")
		if !status.Exists() {
			return "", fmt.Errorf("Invalid result returned. %s", result)
		}
	}

	return status.String(), nil
}

func PollInvoices(token string, lastIndex uint64) ([]*Invoice, error) {
	result, err := lightningConn.Rpc(token, "listinvoices",
		fmt.Sprintf("{\"start\":%d,\"index\":\"updated\"}", lastIndex))

	if err != nil {
		return nil, err
	}

	var invs []*Invoice
	invoices := gjson.Get(result, "result.invoices")
	if !invoices.Exists() {
		return nil, fmt.Errorf("Invalid result returned. %s", result)
	}

	invoices.ForEach(func (key, value gjson.Result) bool {
		inv := &Invoice{}
		err = inv.UnmarshalJSON([]byte(value.Raw))
		if err != nil {
			return false
		}
		invs = append(invs, inv)
		return true
	})

	if err != nil {
		return nil, err
	}

	return invs, nil
}
