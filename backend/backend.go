package backend

//package main

import (
	"dataCollect"
	"net/http"
	"time"

	"github.com/laktek/Stack-on-Go/stackongo"
)

var tags = []string{"google-places-api"}

func refreshCache(r *http.Request) error {
	// Set starting variable parameters
	page := 1
	toDate := time.Now()

	// Adding parameters to request
	params := make(stackongo.Params)
	params.Page(page)
	params.Pagesize(100)
	params.Todate(toDate)
	params.Sort("creation")
	params.Add("accepted", false)
	params.AddVectorized("tagged", tags)

	return dataCollect.Collect(r, params)
}

func newSearch(r *http.Request, params stackongo.Params) ([]byte, error) {
	return dataCollect.Collect(r, params)
}

// for collecting datasets - will remove before launch
/*func main() {
	input, err := Collect(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	ioutil.WriteFile("3-12_dataset.json", input, 640)
}*/
