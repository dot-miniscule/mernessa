package main

import (
	"fmt";
	"flag";
	"io/ioutil";
	"encoding/json";

	"github.com/laktek/Stack-on-Go/stackongo"
)

var filename = flag.String("file", "", "file to read JSON data from")

func main() {
	flag.Parse()

	if *filename == "" {
		fmt.Println("No file listed")
		return
	}

	input, err := ioutil.ReadFile(*filename)
	if err != nil {
		panic(err)
	}
	questions := new(stackongo.Questions)
	if err := json.Unmarshal(input, questions) ; err != nil {
		fmt.Print("Error = ")
		fmt.Println(err.Error())
	} else {
		fmt.Println(len(questions.Items))
		fmt.Println(questions.Has_more)
		fmt.Println(questions.Quota_remaining)
	}
}
