package main

import (
	"io"

	"encoding/json"
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"

	"github.com/captaincodeman/datastore-mapper"
)

type (
	// ExportJson exports JSON to Cloud Storage
	ExportJson struct {
		order   *Order
		encoder *json.Encoder
	}
)

func init() {
	mapper.RegisterJob(&ExportJson{})
}

func (x *ExportJson) Query(r *http.Request) (*mapper.Query, error) {
	q := mapper.NewQuery("order")
	q = q.NamespaceEmpty()
	return q, nil
}

func (x *ExportJson) Make() interface{} {
	x.order = new(Order)
	return x.order
}

func (x *ExportJson) Output(w io.Writer) {
	x.encoder = json.NewEncoder(w)
}

// Next processes the next item
func (x *ExportJson) Next(c context.Context, counters mapper.Counters, key *datastore.Key) error {
	x.order.ID = x.order.Seed
	x.encoder.Encode(x.order)

	return nil
}
