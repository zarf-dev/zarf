# go-presenter

This repo houses a single spot for a presenter abstraction:

```go
type Presenter interface {
    Present(writer io.Writer) error
}
```

I've tended to use this abstraction in multiple projects, but the abstraction itself isn't conceptually coupled to any one project.
This allows for a one-stop-shop to use this abstraction without having to redefine it for each project. Why is this abstraction helpful? 
This abstraction enables a few things:
- you can output bytes without the implication that you need a copy of all of the bytes to write at any one point in time
- separates what is being encoded from the encoding invocation (similar to `json.Encoder`)

