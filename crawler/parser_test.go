package crawler

import (
	"github.com/bitcoinschema/go-bmap"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"os"
	"testing"
)

func TestSubscriptionMAPIsDecodedAfterOpReturn(t *testing.T) {
	// Real MAP subscription transaction from block 946918. With go-bpu 0.2.2
	// and the newer SDK it parsed successfully but silently returned no MAP.
	raw, err := os.ReadFile("testdata/map-op-return.hex")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := transaction.NewTransactionFromHex(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := bmap.NewFromTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.MAP) != 1 {
		t.Fatalf("expected MAP record, got %d", len(parsed.MAP))
	}
}
