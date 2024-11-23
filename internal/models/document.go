package models

import "time"

type Document struct {
	ID        string
	OwnerID   int
	Name      string
	MIME      string
	File      bool
	Public    bool
	CreatedAt time.Time
	Grant     []string
	Data      []byte
}
