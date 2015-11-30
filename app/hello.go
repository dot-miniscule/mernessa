/* 
	This Go Package responds to any request by sending a response containing the message Hello, vanessa.

*/

package hello

import (
	"fmt"
	"net/http"
)


func init() {
	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, Vanessa!")
}
