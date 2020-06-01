package main

import (
	"context"
	"fmt"
	"time"

	"github.com/egorka-gh/sm/tsd"

	_ "github.com/godror/godror"
)

func main() {

	repo, err := tsd.NewRepo("supermag/fhfvbc1999@skont08")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer repo.Close()
	/*
		d, err := repo.LoadDoc(context.Background(), "96242")
		fmt.Printf("doc: %v; err: %s", d, err)
	*/
	docs, err := repo.LoadDocByTime(context.Background(), time.Now().Add(-15*time.Hour), -1)

	fmt.Println(err)
	fmt.Println(len(docs))
}
