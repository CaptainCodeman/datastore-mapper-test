package main

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"time"

	"encoding/gob"
	"net/http"

	"golang.org/x/net/context"

	"github.com/speps/go-hashids"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	"github.com/captaincodeman/datastore-locker"
	"github.com/captaincodeman/datastore-mapper"
)

type (
	// Task is the generator job
	Task struct {
		locker.Lock
		Start   int64 `datastore:"start"`
		Finish  int64 `datastore:"finish"`
		NewOnly bool  `datastore:"new_only"`
	}
)

var (
	h *hashids.HashID
	l *locker.Locker
)

const (
	batchSize = 100
)

func init() {
	gob.Register(&datastore.Key{})

	l, _ = locker.NewLocker(
		locker.AlertOnFailure,
		locker.AlertOnOverwrite,
		locker.MaxRetries(3),
	)

	mapperServer, _ := mapper.NewServer(mapper.DefaultPath)
	http.Handle(mapper.DefaultPath, mapperServer)
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/orders", ordersHandler)
	http.HandleFunc("/_ah/start", startHandler)
	http.Handle("/_ah/generate", l.Handle(generateHandler, taskFactory))

	hd := hashids.NewData()
	h = hashids.NewWithData(hd)
}

func taskFactory() locker.Lockable {
	return new(Task)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "mapper-perf")
}

func ordersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	q := datastore.NewQuery("order").Order("seed").Limit(10)
	b := new(bytes.Buffer)
	for t := q.Run(ctx); ; {
		var o Order
		key, err := t.Next(&o)
		if err == datastore.Done {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(b, "Key=%v\nOrder=%#v\n\n", key, o)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.Copy(w, b)
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	start, _ := strconv.ParseInt(q.Get("start"), 10, 64)
	finish, _ := strconv.ParseInt(q.Get("finish"), 10, 64)
	newOnly, _ := strconv.ParseBool(q.Get("new_only"))

	if finish < start {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	c := appengine.NewContext(r)
	task := &Task{Start: start, Finish: finish, NewOnly: newOnly}
	key := datastore.NewKey(c, "task", "", 1, nil)
	l.Schedule(c, key, task, "/_ah/generate", nil)
}

func generateHandler(c context.Context, r *http.Request, key *datastore.Key, entity locker.Lockable) error {
	task := entity.(*Task)
	task.generate(c)

	if task.Start < task.Finish {
		return l.Schedule(c, key, task, "/_ah/generate", nil)
	}

	return l.Complete(c, key, task)
}

func (t *Task) generate(c context.Context) {
	log.Debugf(c, "generate: %d %d-%d (new_only:%t)", t.Sequence, t.Start, t.Finish, t.NewOnly)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(5)*time.Minute)
	source := generate(ctx, t.Start+1)
	keys := make([]*datastore.Key, 0, batchSize)
	orders := make([]*Order, 0, batchSize)

	for order := range source {
		if order.ID > t.Finish {
			cancel()
			break
		}
		x, _ := h.EncodeInt64([]int64{order.ID})

		key := datastore.NewKey(c, "order", x, 0, nil)
		keys = append(keys, key)
		orders = append(orders, order)

		if len(orders) == batchSize {
			t.processbatch(c, keys, orders)
			keys = nil
			orders = nil
		}

		t.Start = order.ID
	}

	if len(orders) > 0 {
		t.processbatch(c, keys, orders)
	}
}

func (t *Task) processbatch(ctx context.Context, keys []*datastore.Key, orders []*Order) error {
	c, _ := context.WithTimeout(ctx, time.Duration(60)*time.Second)

	var (
		putKeys   []*datastore.Key
		putOrders []*Order
	)
	size := len(orders)

	if t.NewOnly {
		tmp := make([]*Order, size)
		err := datastore.GetMulti(c, keys, tmp)
		if err == nil {
			return nil
		}

		putKeys = make([]*datastore.Key, 0, size)
		putOrders = make([]*Order, 0, size)

		if me, ok := err.(appengine.MultiError); ok {
			for i, merr := range me {
				if merr == nil {
					continue
				}
				if merr == datastore.ErrNoSuchEntity {
					putKeys = append(putKeys, keys[i])
					putOrders = append(putOrders, orders[i])
					log.Warningf(c, "missing %s", keys[i])
				} else {
					log.Errorf(c, "get err: %s %v %T", keys[i], merr, merr)
				}
			}
		} else {
			log.Errorf(c, "getmulti err: %v", err)
			return err
		}
	} else {
		putKeys = keys
		putOrders = orders
	}

	if len(putKeys) > 0 {
		if _, err := datastore.PutMulti(c, putKeys, putOrders); err != nil {
			log.Errorf(c, "put err: %v", err)
			return err
		}
	}

	return nil
}
