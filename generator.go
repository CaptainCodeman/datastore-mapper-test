package main

import (
	"time"

	"math/rand"

	"github.com/syscrusher/fake"
	"golang.org/x/net/context"
)

var (
	from = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	to   = time.Date(2010, 12, 31, 23, 59, 59, 0, time.UTC)
)

func generate(ctx context.Context, seed int64) <-chan *Order {
	out := make(chan *Order)

	go func() {
		defer close(out)

	gen:
		for {
			rand.Seed(seed)
			itemCount := rand.Intn(4) + 1
			items := make([]OrderItem, 0, itemCount)
			currency := fake.CurrencyCode()

			var total int64
			for i := 0; i < itemCount; i++ {
				item := OrderItem{
					Product: rand.Int63(),
					Brand:   fake.Brand(),
					Name:    fake.ProductName(),
					Price: Money{
						Amount:   rand.Int63n(9900) + 99,
						Currency: currency,
					},
					Qty: rand.Int63n(2) + 1,
				}
				item.Total = Money{
					Amount:   item.Price.Amount * item.Qty,
					Currency: currency,
				}
				items = append(items, item)
				total += item.Total.Amount
			}
			order := &Order{
				ID:   seed,
				Date: RandomTime(from, to),
				Address: Address{
					Street:  fake.StreetAddress(),
					City:    fake.City(),
					Region:  fake.StateAbbrev(),
					Postal:  fake.Zip(),
					Country: fake.Country(),
				},
				Items: items,
				Total: Money{
					Amount:   total,
					Currency: currency,
				},
				Seed: seed,
			}

			select {
			case out <- order:
				seed++
			case <-ctx.Done():
				break gen
			}
		}
	}()

	return out
}

func RandomTime(from, to time.Time) time.Time {
	diff := to.Sub(from)
	ns := rand.Int63n(diff.Nanoseconds())
	return from.Add(time.Duration(ns) * time.Nanosecond)
}
