package main

import (
	"encoding/json"
	"fmt"
)

type HaHa interface {
	Ha()
}

type Test struct {
	Name string
	H    HaHa
}

func (t *Test) Ha() {

}

func main() {
	var t HaHa = &Test{Name: "123", H: &Test{Name: "123", H: nil}}
	data, err := json.Marshal(t)
	if err != nil {
		panic(err)
	} else {
		fmt.Println(string(data))
	}
	var h = &Test{}
	if err := json.Unmarshal(data, h); err != nil {
		panic(err)
	}

}
