# gohttp
Simpler go http client

Sample usage:

GET with parameters:
```
cli := http.NewClient()

var ret map[string]interface{}
err := cli.Get(context.Background(),
    "http://example.com",
    http.WithParam("debug", "1),
    http.WithJSONResponse(&ret))
```

POST with body:
```
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
defer cancel()

body = map[string]string{
   "hello": "world",
}
err := cli.Post(ctx, "http://example.com",
    http.WithJSONBody(body),
    http.WithHeader("Authorization", "Bearer my-token"))
```
