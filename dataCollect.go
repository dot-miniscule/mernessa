package main

import (
    "fmt";
    "time";
    "flag";
    "io/ioutil";
    "encoding/json";

    "github.com/laktek/Stack-on-Go/stackongo"
//    "appengine/urlfetch"
)

var appInfo = struct {
	client_id       string
	redirect_uri    string
	client_secret   string
	key             string
	options         map[string]string
	tags            []string
	filters         string
}{
	client_id       : "6029",
	redirect_uri    : "https://stackexchange.com/oauth/login_success",
	client_secret   : "ymefu0zw2TIULhSTM03qyg((",
	key             : "nHI22oWrBEsUN8kHe4ARsQ((",
	options         : map[string]string {
                        "scope": "no_expiry",
                      },
	tags            : []string{"google-places-api"},
	filters	        : "!5RCKNP5Mc6PLxOe3ChJgVk5f4", // Includes: 
                                                    //	- Wrapper: backoff, error_id, error_message, error_name, 
                                                    //             has_more, items, quota_remaining
                                                    //	- Question: body, creation_date, link, question_id, title
}

var delay = 1 * time.Second
var week = (24 * 7) * time.Hour

var filename = flag.String("file", "", "set where to write the data to")

func main() {
    flag.Parse()
    if *filename == "" {
        fmt.Println("No file specified")
        return
    }
/*    c := appengine.NewContext(r)
    ut := &urlfetch.Transport{Context: c}
    stackongo.SetTransport(ut)
*/
    session := stackongo.NewSession("stackoverflow")

    // Set starting variable parameters
    page := 1
    toDate := time.Now()
    fromDate := toDate.Add(-1*week + (24*time.Hour))

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
        fmt.Println(err.Error())
    }

    output, _ := json.Marshal(questions)
    if err := ioutil.WriteFile(*filename, output, 0644) ; err != nil {
        fmt.Println(err.Error())
    }
}
