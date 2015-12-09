package dataCollect

import (
	"net/http"
	"time"

	"github.com/laktek/Stack-on-Go/stackongo"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
)

var appInfo = struct {
	client_id     string
	redirect_uri  string
	client_secret string
	key           string
	options       map[string]string
	filters       string
}{
	client_id:     "6029",
	redirect_uri:  "https://stackexchange.com/oauth/login_success",
	client_secret: "ymefu0zw2TIULhSTM03qyg((",
	key:           "nHI22oWrBEsUN8kHe4ARsQ((",

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

func Collect(r *http.Request, params stackongo.Params) (*stackongo.Questions, error) {
	c := appengine.NewContext(r)
	ut := &urlfetch.Transport{Context: c}
	stackongo.SetTransport(ut)

	session := stackongo.NewSession("stackoverflow")

	params.Add("key", appInfo.key)
	params.Add("filter", appInfo.filters)
	params.Add("site", "stackoverflow")

	return session.AllQuestions(params)
}
