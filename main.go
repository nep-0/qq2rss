package main

import (
	"qq2rss/server"
	"time"

	"github.com/google/uuid"
)

func main() {
	s, err := server.NewServer(server.Config{
		Title:       "UESTC LUG",
		Link:        "https://qm.qq.com/q/itSCyURda8",
		Description: "UESTC LUG QQ group links shared",
		AuthorName:  "UESTC LUG",
		AuthorEmail: "placeholder@example.com",
		StoragePath: "store.json",
		MaxItems:    20,
	})
	if err != nil {
		panic(err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	s.AddItem(server.Item{
		ID:          id.String(),
		Title:       "test",
		Link:        "https://qm.qq.com/q/itSCyURda8",
		Description: "test description",
		Content:     "test content",
		AuthorName:  "Test Author",
		AuthorEmail: "placeholder@example.com",
		Created:     time.Now(),
	})

	s.Start(":8080")
}
