package crawler

import (
	"context"
	"log"
	"os"
	"time"
	"unicode/utf8"

	"fmt"

	"github.com/GorillaPool/go-junglebus"
	"github.com/GorillaPool/go-junglebus/models"
	"github.com/bitcoinschema/go-bmap"
	"github.com/libsv/go-bt/v2"
	"github.com/rohenaz/go-bmap-indexer/config"
	"github.com/rohenaz/go-bmap-indexer/persist"
	"github.com/rohenaz/go-bmap-indexer/state"
	"github.com/ttacon/chalk"
	"go.mongodb.org/mongo-driver/bson"
)

// var wgs map[uint32]*sync.WaitGroup
var cancelChannel chan int
var eventChannel chan *Event

func SyncBlocks(height int) (newBlock int) {
	// Setup crawl timer
	crawlStart := time.Now()

	// Crawl will mutate currentBlock
	newBlock = Crawl(height)

	// Crawl complete
	diff := time.Since(crawlStart).Seconds()

	// TODO: I believe if we get here crawl has actually died
	fmt.Printf("Junglebus closed after %fs\nBlock height: %d\n", diff, height)
	return
}

type BlockState struct {
	Height  int
	Retries int
}

type CrawlState struct {
	Height int
	Blocks []BlockState
}

type Event struct {
	Type        string
	Error       error
	Height      uint32
	Time        uint32
	Id          string
	Transaction []byte
	Status      string
}

func init() {
	// TODO: Is this needed?
	// wgs = make(map[uint32]*sync.WaitGroup)
	// cancelChannel = make(chan int)
	eventChannel = make(chan *Event, 1000000) // Buffered channel
}

// Crawl loops over the new bmap transactions since the given block height
func Crawl(height int) (newHeight int) {

	// readyFiles := make(chan string, 1000) // Adjust buffer size as needed
	// make the first waitgroup for the initial block
	// hereafter we will add these in block done event
	// wgs[uint32(height)] = &sync.WaitGroup{}

	junglebusClient, err := junglebus.New(
		junglebus.WithHTTP("https://junglebus.gorillapool.io"),
	)
	if err != nil {
		log.Fatalln(err.Error())
	}

	subscriptionID := config.SubscriptionID

	// get from block from block.tmp
	fromBlock := uint64(config.FromBlock)

	lastBlock := uint64(state.LoadProgress())

	if lastBlock > fromBlock {
		fromBlock = lastBlock
	}

	eventHandler := junglebus.EventHandler{
		// Mined tx callback
		OnTransaction: func(tx *models.TransactionResponse) {
			log.Printf("[TXa]: %d: %v", tx.BlockHeight, tx.Id)

			eventChannel <- &Event{
				Type:        "transaction",
				Height:      tx.BlockHeight,
				Time:        tx.BlockTime,
				Transaction: tx.Transaction,
				Id:          tx.Id,
			}
		},
		// Mempool tx callback
		OnMempool: func(tx *models.TransactionResponse) {
			log.Printf("[MEMa]: %d: %v", tx.BlockHeight, tx.Id)

			eventChannel <- &Event{
				Type:        "mempool",
				Transaction: tx.Transaction,
				Id:          tx.Id,
			}
		},
		OnStatus: func(status *models.ControlResponse) {
			log.Printf("[STATa]: %d: %v", status.Block, status.Status)

			eventChannel <- &Event{
				Type:   "status",
				Height: status.Block,
				Status: status.Status,
			}
		},
		OnError: func(err error) {
			eventChannel <- &Event{Type: "error", Error: err}
		},
	}

	fmt.Printf("Initializing from block %d\n", fromBlock)

	var subscription *junglebus.Subscription
	if subscription, err = junglebusClient.Subscribe(context.Background(), subscriptionID, fromBlock, eventHandler); err != nil {
		log.Printf("ERROR: failed getting subscription %s", err.Error())
	}

	if err != nil {
		log.Printf("ERROR: failed getting subscription %s", err.Error())
		unsubscribeError := subscription.Unsubscribe()

		if err = subscription.Unsubscribe(); unsubscribeError != nil {
			log.Printf("ERROR: failed unsubscribing %s", err.Error())
		}
	}

	// wait indefinitely to make sure we dont stop
	// before more mempool txs come in
	go eventListener(subscription)

	// have a channel here listen for the stop signal, decrement the waitgroup
	// and return the new block height to resubscribe from

	// Print tx line to stdout
	// if err != nil {
	// 	fmt.Println(err)
	// }

	return
}

func CancelCrawl(newBlockHeight int) {
	log.Printf("%s[INFO]: Canceling crawl at block %d%s\n", chalk.Yellow, newBlockHeight, chalk.Reset)
	cancelChannel <- newBlockHeight
}

func processTransactionEvent(rawtx []byte, blockHeight uint32, blockTime uint32) {
	if len(rawtx) > 0 {
		// log.Printf("[TX]: %d: %s | Data Length: %d", blockHeight, tx.Id, len(tx.Transaction))
		t, err := bt.NewTxFromBytes(rawtx)
		if err != nil {
			log.Printf("[ERROR]: %v", err)
			return
		}
		bmapTx, err := bmap.NewFromTx(t)
		if err != nil {
			log.Printf("[ERROR]: %v", err)
			return
		}

		bmapTx.Blk.I = blockHeight
		bmapTx.Blk.T = blockTime

		// log.Printf("[BMAP]: %d: %s | Data Length: %d | First 10 bytes: %x", tx.BlockHeight, bmapTx.Tx.Tx.H, len(tx.Transaction), tx.Transaction[:10])

		processTx(bmapTx)
	}
}

// func processMempoolEvent(rawtx []byte) {
// 	log.Printf("[MEMPOOL TX]: %s", tx.Id)
// 	t, err := bt.NewTxFromBytes(rawtx)
// 	if err != nil {
// 		log.Printf("[ERROR]: %v", err)
// 		return
// 	}
// 	bmapTx, err := bmap.NewFromTx(t)
// 	if err != nil {
// 		log.Printf("[ERROR]: %v", err)
// 		return
// 	}
// 	log.Printf("[MEMPOOL BMAP]: %d: %v", bmapTx.Blk.I, bmapTx.Tx.Tx.H)
// }

func processBlockDoneEvent(height uint32) {
	// log.Printf("[BLOCK DONE]: %v, %d", status, status.Block)

	filename := fmt.Sprintf("data/%d.json", height)

	// // check if the file exists at path
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("Block %d done with %d txs", height, 0)
		return
	}

	// // change file to readonly (ready for ingestion)
	// // err := os.Chmod(filename, 0444) // read-only permissions
	// // if err != nil {
	// // 	log.Printf("Error changing permissions for %s: %v", filename, err)
	// // }

	ingest(filename)
	state.SaveProgress(height)
	if config.DeleteAfterIngest {
		err := os.Remove(filename)
		if err != nil {
			fmt.Printf("%s%s %s: %v%s\n", chalk.Cyan, "Error deleting file", filename, err, chalk.Reset)
		}
	}
}

func processTx(bmapData *bmap.Tx) {
	bsonData := bson.M{
		"_id": bmapData.Tx.Tx.H,
		"tx":  bmapData.Tx.Tx,
		"blk": bmapData.Tx.Blk,
		// go equivalent of Math.round(new Date().getTime() / 1000)
		"timestamp": time.Now().Unix(),
	}

	if bmapData.AIP != nil {
		bsonData["AIP"] = bmapData.AIP
	}

	if bmapData.BAP != nil {
		bsonData["BAP"] = bmapData.BAP
	}

	if bmapData.Ord != nil {
		bsonData["Ord"] = bmapData.Ord
	}

	if bmapData.B != nil {
		bsonData["B"] = bmapData.B
	}

	if bmapData.BOOST != nil {
		bsonData["BOOST"] = bmapData.BOOST
	}

	if bmapData.MAP == nil {
		log.Println("No MAP data.")
		return
	}
	bsonData["MAP"] = bmapData.MAP
	if _, ok := bmapData.MAP[0]["type"].(string); !ok {
		// log.Println("Error: MAP 'type' key does not exist.")
		return
	}
	if _, ok := bmapData.MAP[0]["app"].(string); !ok {
		// log.Println("Error: MAP 'app' key does not exist.")
		return
	}

	for key, value := range bsonData {
		if str, ok := value.(string); ok {
			if !utf8.ValidString(str) {
				log.Printf("Invalid UTF-8 detected in key %s: %s", key, str)
				return
			}
		}
	}

	// 	Write to local filesystem
	err := persist.SaveLine(fmt.Sprintf("data/%d.json", bmapData.Blk.I), bsonData)
	if err != nil {
		log.Printf("[WRITE ERROR]: %v", err)
		return
	}
}
