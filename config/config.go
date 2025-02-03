package config

var BitcoinSchemaTypes = []string{"friend", "like", "repost", "post", "message"}

// There are config constants
const (
	SkipSPV           = true
	MinerAPIEndpoint  = "https://mapi.gorillapool.iom/mapi/tx/"
	JunglebusEndpoint = "https://junglebus.gorillapool.io/"
	FromBlock         = 817000                            // "Welcome to the Future" post = 574287
	BockSyncRetries   = 5                                 // number of retries before block is marked failed
	DeleteAfterIngest = false                             // delete json data files after ingesting to db. If using p2p this will effective disable seeding (jerk)
	EnableP2P         = true                              // enable p2p layer
	OutputTypes       = "friend,like,repost,post,message" // you can adjust these to change the output types you want to index
)
