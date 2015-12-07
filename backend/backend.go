package backend

//package main

import (
	"dataCollect"
	"net/http"
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/laktek/Stack-on-Go/stackongo"
)

var (
	tags    = []string{"google-places-api"}
	appInfo = struct {
		client_id     string
		redirect_uri  string
		client_secret string
		key           string
		options       map[string]string
		filters       string
	}{
		client_id:     "6029",
		redirect_uri:  "https://localhost:8080/home",
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
	session = new(stackongo.Session)
)

func NewSession(r *http.Request) *stackongo.Session {
	c := appengine.NewContext(r)
	ut := &urlfetch.Transport{Context: c}
	stackongo.SetTransport(ut)

	session = stackongo.NewSession("stackoverflow")
	return session
}

func RefreshCache(r *http.Request) (*stackongo.Questions, error) {
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

func NewSearch(r *http.Request, params stackongo.Params) (*stackongo.Questions, error) {
	return dataCollect.Collect(r, params)
}

func AuthURL(options map[string]string) string {
	return stackongo.AuthURL(appInfo.client_id, appInfo.redirect_uri, options)
}

func ObtainAccessToken(code string) (map[string]string, error) {
	return stackongo.ObtainAccessToken(appInfo.client_id, appInfo.client_secret, code, appInfo.redirect_uri)
}

func AuthenticatedUser(params map[string]string, auth map[string]string) (stackongo.User, error) {
	return session.AuthenticatedUser(params, auth)
}

func GetUser(user_id int, params map[string]string) (stackongo.User, error) {
	users, err := session.GetUsers([]int{user_id}, params)
	if err != nil {
		return stackongo.User{}, err
	}
	return users.Items[0], nil
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
