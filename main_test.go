package main

import (
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
)

func TestDS(t *testing.T) {
	t.Skip()

	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer done()

	c, _ := context.WithTimeout(context.TODO(), time.Duration(10)*time.Millisecond)

	orders := generate(c, 0)
	for order := range orders {
		key := datastore.NewIncompleteKey(ctx, "order", nil)
		datastore.Put(ctx, key, order)
	}
}
