package dataCollect

import (
	"time"

	"github.com/laktek/Stack-on-Go/stackongo"
)

type AppDetails struct {
	Client_id       string
	Redirect_uri    string
	Client_secret   string
	Key             string
	Options         map[string]string
	Filters         string
	Quota_remaining int
}

var delay = 1 * time.Second
var week = (24 * 7) * time.Hour

func Collect(session *stackongo.Session, appInfo AppDetails, params stackongo.Params) (*stackongo.Questions, error) {
	params.Add("key", appInfo.Key)
	params.Add("filter", appInfo.Filters)
	params.Add("site", "stackoverflow")

	return session.AllQuestions(params)
}
