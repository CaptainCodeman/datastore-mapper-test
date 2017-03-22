package main

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"

	"github.com/captaincodeman/datastore-mapper"
)

type (
	// DeleteData deletes all the data to cleanup
	DeleteData struct {
		keys []*datastore.Key
	}
)

func init() {
	mapper.RegisterJob(&DeleteData{})
}

func (x *DeleteData) Query(r *http.Request) (*mapper.Query, error) {
	q := mapper.NewQuery("order")
	q = q.NamespaceEmpty()
	return q, nil
}

func (x *DeleteData) SliceStarted(c context.Context, id string, namespace string, shard, slice int) {
	x.keys = make([]*datastore.Key, 0, batchSize)
}

// Next processes the next item
func (x *DeleteData) Next(c context.Context, counters mapper.Counters, key *datastore.Key) error {
	x.keys = append(x.keys, key)
	if len(x.keys) == cap(x.keys) {
		x.processBatch(c)
	}
	return nil
}

func (x *DeleteData) SliceCompleted(c context.Context, id string, namespace string, shard, slice int) {
	x.processBatch(c)
}

func (x *DeleteData) processBatch(c context.Context) {
	datastore.DeleteMulti(c, x.keys)
	x.keys = nil
}
