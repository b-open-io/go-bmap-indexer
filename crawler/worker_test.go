package crawler

import (
	"go.mongodb.org/mongo-driver/bson"
	"testing"
)

func TestSaveTransactionSkipsNonIndexableRecords(t *testing.T) {
	// These must return before accessing the database, including the nil MAP
	// record that previously crashed the production worker.
	for _, doc := range []bson.M{
		nil, {}, {"MAP": nil}, {"MAP": []interface{}{}},
		{"collection": ""}, {"collection": "post"},
		{"collection": "post", "_id": 123},
	} {
		saveTransaction(doc)
	}
}
