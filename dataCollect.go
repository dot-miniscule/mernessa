package dataCollect

import (
	"encoding/json"
	"time"

	"github.com/laktek/Stack-on-Go/stackongo"
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

	filters: "!5RCKNP5Mc6PLxOe3ChJgVk5f4",
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

func collectData() ([]byte, error) {
	session := stackongo.NewSession("stackoverflow")

	// Set starting variable parameters
	page := 1
	toDate := time.Now()
	fromDate := toDate.Add(-1*week + (24 * time.Hour))

	// Adding parameters to request
	params := make(stackongo.Params)
	params.Add("key", appInfo.key)
	params.Page(page)
	params.Pagesize(5)
	params.Fromdate(fromDate)
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
