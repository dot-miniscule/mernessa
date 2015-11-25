package main

import (
    "fmt";
    "time";

    "github.com/laktek/Stack-on-Go/stackongo"
//  "appengine/urlfetch"
)

var delay = 1 * time.Second
var week = int64(60 * 60 * 24 * 7)

func main() {
    /*c := appengine.NewContext(r)
    ut := &urlfetch.Transport{Context: c}

    stackongo.SetTransport(ut)
*/
    key := "nHI22oWrBEsUN8kHe4ARsQ(("
    tags := []string{"google-places-api"}
    session := stackongo.NewSession("stackoverflow")
    fromDate := time.Now().Unix() - week
    toDate := time.Now().Unix()

    params := make(stackongo.Params)
    params.AddVectorized("tagged", tags)
    params.Add("accepted", false)
    params.Add("page", 1)
    params.Add("pagesize", 5)

    params.Add("fromdate", fromDate)
    params.Add("todate", toDate)

    for i := 0; i < 5; i++ {
        questions, err := session.UnansweredQuestions(params)
	    if err != nil {
		    fmt.Println("*****************\n" + err.Error()+ "\n*****************")
	    }
        fromDate -= week
        toDate -= week
        params.Set("fromdate", fromDate)
        params.Set("toDate", toDate)
        for _, question := range questions.Items {
            fmt.Println(question.Title)
            fmt.Printf("Link: %v\n\n", question.Link)
        }
        time.Sleep(delay)
    }
}
