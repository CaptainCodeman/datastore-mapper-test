package main

import (
	"time"
)

type (
	// Order represents an order
	Order struct {
		ID      int64       `json:"id" datastore:"-"`
		Date    time.Time   `json:"date" datastore:"date"`
		Total   Money       `json:"total" datastore:"total,noindex"`
		Items   []OrderItem `json:"items" datastore:"items,noindex"`
		Address Address     `json:"address" datastore:"address,noindex"`
		Seed    int64       `json:"-" datastore:"seed"`
	}

	// OrderItem represents an item within an order
	OrderItem struct {
		Product int64  `json:"product" datastore:"product,noindex"`
		Brand   string `json:"brand" datastore:"brand,noindex"`
		Name    string `json:"name" datastore:"name,noindex"`
		Price   Money  `json:"price" datastore:"price,noindex"`
		Qty     int64  `json:"qty" datastore:"qty,noindex"`
		Total   Money  `json:"total" datastore:"total,noindex"`
	}

	// Money represents a monetary value
	Money struct {
		Amount   int64  `json:"amount" datastore:"amount,noindex"`
		Currency string `json:"currency" datastore:"currency,noindex"`
	}

	// Address represents a postal address
	Address struct {
		Street  string `json:"street" datastore:"street,noindex"`
		City    string `json:"city" datastore:"city,noindex"`
		Region  string `json:"region" datastore:"region,noindex"`
		Postal  string `json:"postal" datastore:"postal,noindex"`
		Country string `json:"country" datastore:"country,noindex"`
	}
)
