package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

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
	d, err := repo.LoadDoc(context.Background(), "96350")
	fmt.Printf("doc: %v; err: %s", d, err)
	m := message{
		Message: "action",
		Records: make([]item, 0, len(d.Items)),
	}
	for _, it := range d.Items {
		id, _ := strconv.Atoi(d.ProcessID)
		rec := item{
			ID:        id,
			Docnum:    fmt.Sprintf("Приход по заказу %s", d.BaseDoc),
			BSO:       d.PaperDoc,
			TTN:       d.ResultDoc,
			Vendor:    d.Vendor,
			SKU:       it.Article,
			Rectime:   it.EventTime.Format("15:04:05"),
			Article:   it.Article,
			Barcode:   it.Barcode,
			Qtt:       it.Qtt,
			Pack:      it.Pack,
			Date:      it.EventTime.Format("02.01.2006"),
			StartTime: d.StartTime.Format("15:04:05"),
			EndTime:   d.EndTime.Format("15:04:05"),
			Person:    d.UserName,
		}
		m.Records = append(m.Records, rec)
	}
	fmt.Println()
	fmt.Println("json")
	b, err := json.Marshal(m)
	os.Stdout.Write(b)
}

type message struct {
	Message string `json:"message"`
	Records []item `json:"records"`
}

type item struct {
	ID        int     `json:"id"`
	Docnum    string  `json:"docnum"`
	BSO       string  `json:"bso"`
	TTN       string  `json:"ttn"`
	Vendor    string  `json:"vendor"`
	SKU       string  `json:"sku"`
	Rectime   string  `json:"skurectime"`
	Article   string  `json:"partnum"`
	Barcode   string  `json:"ean"`
	Qtt       float64 `json:"qty"`
	Pack      float64 `json:"package"`
	Date      string  `json:"date"`
	StartTime string  `json:"begintime"`
	EndTime   string  `json:"endtime"`
	Person    string  `json:"person"`
}
