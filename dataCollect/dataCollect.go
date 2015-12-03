package dataCollect

//package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/laktek/Stack-on-Go/stackongo"

	"appengine"
	"appengine/urlfetch"
)

var appInfo = struct {
	client_id     string
	redirect_uri  string
	client_secret string
	key           string
	options       map[string]string
	tags          []string
	filters       string
}{
	client_id:     "6029",
	redirect_uri:  "https://stackexchange.com/oauth/login_success",
	client_secret: "ymefu0zw2TIULhSTM03qyg((",
	key:           "nHI22oWrBEsUN8kHe4ARsQ((",
	tags:          []string{"google-places-api"},

	filters: "!846.hCHXJtBDPB1pe-0GnXRad1cyWBkz(ithJ4-ztkzynXtQgKxaGE4ry3jiLpLNWv5",
	// Filters include:
	//	- Wrapper: backoff, error_id, error_message, error_name,
	//             has_more, items, quota_remaining
	//	- Question: body, creation_date, link, question_id, title

	options: map[string]string{
		"scope": "no_expiry",
	},
}

var delay = 1 * time.Second
var week = (24 * 7) * time.Hour

func Collect(r *http.Request) ([]byte, error) {
	c := appengine.NewContext(r)
	ut := &urlfetch.Transport{Context: c}
	stackongo.SetTransport(ut)

	session := stackongo.NewSession("stackoverflow")

	// Set starting variable parameters
	page := 1
	toDate := time.Now()

	// Adding parameters to request
	params := make(stackongo.Params)
	params.Add("key", appInfo.key)
	params.Page(page)
	params.Pagesize(100)
	params.Todate(toDate)
	params.Sort("creation")
	params.Add("accepted", false)
	params.AddVectorized("tagged", appInfo.tags)
	params.Add("site", "stackoverflow")
	params.Add("filter", appInfo.filters)

	questions, err := session.AllQuestions(params)
	if err != nil {
		return nil, err
	}

	return json.Marshal(questions)
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
