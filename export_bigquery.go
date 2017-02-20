package main

import (
	"strconv"
	"time"

	"net/http"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/bigquery/v2"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	"github.com/captaincodeman/datastore-mapper"
)

type (
	// ExportBigQuery exports data to directly to BigQuery
	ExportBigQuery struct {
		order *Order
		appID string
		bq    *bigquery.Service
		rows  []*bigquery.TableDataInsertAllRequestRows
	}
)

func init() {
	mapper.RegisterJob(&ExportBigQuery{})
}

func (x *ExportBigQuery) Query(r *http.Request) (*mapper.Query, error) {
	q := mapper.NewQuery("order")
	q = q.NamespaceEmpty()
	return q, nil
}

// Make creates the entity to load into
func (x *ExportBigQuery) Make() interface{} {
	x.order = new(Order)
	return x.order
}

func (x *ExportBigQuery) JobStarted(c context.Context, id string) {
	ensureTables(c)
}

func (x *ExportBigQuery) SliceStarted(c context.Context, id string, namespace string, shard, slice int) {
	x.bq, _ = bigqueryService(c)
	x.appID = appengine.AppID(c)
	x.rows = make([]*bigquery.TableDataInsertAllRequestRows, 0, 100)
}

func (x *ExportBigQuery) SliceCompleted(c context.Context, id string, namespace string, shard, slice int) {
	x.insertBatch(c)
}

func (x *ExportBigQuery) JobCompleted(c context.Context, id string) {
}

// Next processes the next item
func (x *ExportBigQuery) Next(c context.Context, counters mapper.Counters, key *datastore.Key) error {
	x.order.ID = x.order.Seed

	var items []bigquery.JsonValue
	for _, item := range x.order.Items {
		items = append(items, map[string]bigquery.JsonValue{
			"product": item.Product,
			"brand":   item.Brand,
			"name":    item.Name,
			"price": map[string]bigquery.JsonValue{
				"amount":   item.Price.Amount,
				"currency": item.Price.Currency,
			},
			"qty": item.Qty,
			"total": map[string]bigquery.JsonValue{
				"amount":   item.Total.Amount,
				"currency": item.Total.Currency,
			},
		})
	}

	x.rows = append(x.rows, &bigquery.TableDataInsertAllRequestRows{
		InsertId: strconv.FormatInt(x.order.ID, 10),
		Json: map[string]bigquery.JsonValue{
			"id":   x.order.ID,
			"date": x.order.Date,
			"total": map[string]bigquery.JsonValue{
				"amount":   x.order.Total.Amount,
				"currency": x.order.Total.Currency,
			},
			"items": items,
			"address": map[string]bigquery.JsonValue{
				"street":  x.order.Address.Street,
				"city":    x.order.Address.City,
				"region":  x.order.Address.Region,
				"postal":  x.order.Address.Postal,
				"country": x.order.Address.Country,
			},
		},
	})

	if len(x.rows) == cap(x.rows) {
		x.insertBatch(c)
	}

	return nil
}

func (x *ExportBigQuery) insertBatch(c context.Context) {
	for len(x.rows) > 0 {
		delay := 2
		attempts := 10
		var di *bigquery.TableDataInsertAllResponse
		if err := Do(func(attempt int) (bool, error) {
			var err error
			di, err = x.bq.Tabledata.InsertAll(x.appID, "test", "orders", &bigquery.TableDataInsertAllRequest{
				Rows:            x.rows,
				SkipInvalidRows: true,
			}).Context(c).Do()

			if err != nil {
				time.Sleep(time.Duration(delay) * time.Millisecond)
				delay *= 2
			}
			return attempt < attempts, err
		}); err != nil {
			log.Errorf(c, "error inserting %v", err)
			return
		}

		if len(di.InsertErrors) > 0 {
			failedRows := make([]*bigquery.TableDataInsertAllRequestRows, 0, len(di.InsertErrors))
			for _, rerr := range di.InsertErrors {
				for _, x := range rerr.Errors {
					log.Errorf(c, "row %d failed %v %v %v", rerr.Index, x.Reason, x.Message, x.Location)
				}
				failedRows = append(failedRows, x.rows[rerr.Index])
			}
			x.rows = failedRows
			time.Sleep(time.Duration(1000) * time.Millisecond)
		} else {
			x.rows = nil
		}
	}
}

func bigqueryService(c context.Context) (*bigquery.Service, error) {
	token := google.AppEngineTokenSource(c, bigquery.BigqueryScope)
	client := oauth2.NewClient(c, token)
	service, err := bigquery.New(client)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func ensureTables(c context.Context) {
	bq, _ := bigqueryService(c)
	appID := appengine.AppID(c)

	bq.Datasets.Insert(appID, &bigquery.Dataset{
		DatasetReference: &bigquery.DatasetReference{
			ProjectId: appID,
			DatasetId: "test",
		},
		Description:  "Test dataset",
		FriendlyName: "Test",
		Location:     "US",
	}).Context(c).Do()

	bq.Tables.Insert(appID, "test", &bigquery.Table{
		TableReference: &bigquery.TableReference{
			ProjectId: appID,
			DatasetId: "test",
			TableId:   "orders",
		},
		Description:  "Test table",
		FriendlyName: "Orders",
		Schema: &bigquery.TableSchema{
			Fields: []*bigquery.TableFieldSchema{
				{
					Name:        "id",
					Type:        "INTEGER",
					Description: "Unique ID",
					Mode:        "REQUIRED",
				},
				{
					Name:        "date",
					Type:        "TIMESTAMP",
					Description: "Date ordered",
					Mode:        "REQUIRED",
				},
				{
					Name:        "total",
					Type:        "RECORD",
					Description: "Total",
					Mode:        "REQUIRED",
					Fields: []*bigquery.TableFieldSchema{
						{
							Name:        "amount",
							Type:        "INTEGER",
							Description: "Amount",
							Mode:        "REQUIRED",
						},
						{
							Name:        "currency",
							Type:        "STRING",
							Description: "Currency",
							Mode:        "REQUIRED",
						},
					},
				},
				{
					Name:        "items",
					Type:        "RECORD",
					Description: "Items",
					Mode:        "REPEATED",
					Fields: []*bigquery.TableFieldSchema{
						{
							Name:        "product",
							Type:        "INTEGER",
							Description: "Product",
							Mode:        "REQUIRED",
						},
						{
							Name:        "brand",
							Type:        "STRING",
							Description: "Brand",
							Mode:        "REQUIRED",
						},
						{
							Name:        "name",
							Type:        "STRING",
							Description: "Name",
							Mode:        "REQUIRED",
						},
						{
							Name:        "price",
							Type:        "RECORD",
							Description: "Price",
							Mode:        "REQUIRED",
							Fields: []*bigquery.TableFieldSchema{
								{
									Name:        "amount",
									Type:        "INTEGER",
									Description: "Amount",
									Mode:        "REQUIRED",
								},
								{
									Name:        "currency",
									Type:        "STRING",
									Description: "Currency",
									Mode:        "REQUIRED",
								},
							},
						},
						{
							Name:        "qty",
							Type:        "INTEGER",
							Description: "Qty",
							Mode:        "REQUIRED",
						},
						{
							Name:        "total",
							Type:        "RECORD",
							Description: "Total",
							Mode:        "REQUIRED",
							Fields: []*bigquery.TableFieldSchema{
								{
									Name:        "amount",
									Type:        "INTEGER",
									Description: "Amount",
									Mode:        "REQUIRED",
								},
								{
									Name:        "currency",
									Type:        "STRING",
									Description: "Currency",
									Mode:        "REQUIRED",
								},
							},
						},
					},
				},
				{
					Name:        "address",
					Type:        "RECORD",
					Description: "Address",
					Mode:        "REQUIRED",
					Fields: []*bigquery.TableFieldSchema{
						{
							Name:        "street",
							Type:        "STRING",
							Description: "Street",
							Mode:        "REQUIRED",
						},
						{
							Name:        "city",
							Type:        "STRING",
							Description: "City",
							Mode:        "REQUIRED",
						},
						{
							Name:        "region",
							Type:        "STRING",
							Description: "Region",
							Mode:        "REQUIRED",
						},
						{
							Name:        "postal",
							Type:        "STRING",
							Description: "Postal",
							Mode:        "REQUIRED",
						},
						{
							Name:        "country",
							Type:        "STRING",
							Description: "Country",
							Mode:        "REQUIRED",
						},
					},
				},
			},
		},
	}).Context(c).Do()
}
