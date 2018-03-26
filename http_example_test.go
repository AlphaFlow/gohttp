package http_test

import (
	"context"
	"fmt"

	"github.com/alphaflow/gohttp"
)

func ExampleClient_Get() {
	cli := http.NewClient()
	err := cli.Get(context.Background(), "http://www.google.com", http.WithParam("debug", "1"))
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("Done")
	}
	// Output: Done
}
