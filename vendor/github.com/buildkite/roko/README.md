# Roko

[![Go Reference](https://pkg.go.dev/badge/github.com/buildkite/roko.svg)](https://pkg.go.dev/github.com/buildkite/roko)
[![Build status](https://badge.buildkite.com/f546789abb4ecc61654da783bdcc05965789a4834958b178ec.svg)](https://buildkite.com/buildkite/roko)

A Lightweight, Configurable, Easy-to-use Retry Library for Go

## Installation

To install, run

```
go get -u github.com/buildkite/roko
```

This will add Roko to your go.mod file, and make it available for use in your project.

## Usage

Roko allows you to configure how your application should respond to operations that can fail. Its core interface is the **Retrier**, which allows you tell you application how, and under what circumstances, it should retry an operation.

Let's say we have some operation that we want to perform:
```Go
func canFail() error {
  // ...
}
```

and if it fails, we want it to retry every 5 seconds, and give up after 3 tries. To do this, we can configure a retrier, and then perform our operation using the `roko.Retrier.Do()` function:
```Go
r := roko.NewRetrier(
  roko.WithMaxAttempts(3),                           // Only try 3 times, then give up
  roko.WithStrategy(roko.Constant(5 * time.Second)), // Wait 5 seconds between attempts
)

err := r.Do(func(r *roko.Retrier) error {
  return canFail()
})
```

In this situation, we'll try to run the `canFail` function, and if it returns an error, we'll wait 5 seconds, then try again. If `canFail` returns an error after hitting its max attempt count, `r.Do` will return that error. If `canFail` succeeds (ie it doesn't return an error), r.Do will return nil.

### Giving up early

Sometimes, an error that your operation returns might not be recoverable, so we don't want to retry it. In this case, we can use the `roko.Retrier.Break` function. `Break()` instructs the retrier to halt after this run - note that it **doesn't immediately halt operation**.

```Go
r := roko.NewRetrier(
  roko.WithMaxAttempts(3),                           // Only try 3 times, then give up
  roko.WithStrategy(roko.Constant(5 * time.Second)), // Wait 5 seconds between attempts
)

err := r.Do(func(r *roko.Retrier) error {
  err := canFail()
  if err.Is(errorUnrecoverable) {
    r.Break()  // Give up, we can't recover from this error
    return err // We still need to return from this function, Break() doesn't halt this callback
    // return nil would be appropriate too, if we don't want to handle this error further
  }
})
```

In this example, if `canFail()` returns an unrecoverable error, the result returned by the `r.Do()` call is the unrecoverable error.

### Never give up!

Alternatively (or as well as!), you might want your retrier to never give up, and continue trying until it eventually succeeds. Roko can facilitate this through the `TryForever()` option.

```Go
r := roko.NewRetrier(
  roko.TryForever(),
  roko.WithStrategy(roko.Constant(5 * time.Second)), // Wait 5 seconds between attempts
)

err := r.Do(func(r *roko.Retrier) error {
  return canFail()
})
```

This will try to perform `canFail()` until it eventaually succeeds.

Note that the `Break()` method mentioned above still works when `TryForever()` is enabled - this allows you to still exit when an unrecoverable error comes along.

### Jitter

In order to avoid a thundering herd problem, roko can be configured to add jitter to its retry interval calculations. When jitter is used, the interval calulator will add a random length of time up to one second to each interval calculation.

```Go
r := roko.NewRetrier(
  roko.WithMaxAttempts(3),                           // Only try 3 times, then give up
  roko.WithJitter()                                  // Add up to a second of jitter
  roko.WithStrategy(roko.Constant(5 * time.Second)), // Wait 5ish seconds between attempts
)

err := r.Do(func(r *roko.Retrier) error {
  return canFail()
})
```

In this example, everything is the same as the first example, but instead of always waiting 5 seconds, the retrier will wait for a random interval between 5 and 6 seconds. This can help reduce resource contention.

### Exponential Backoff

If a constant retry strategy isn't to your liking, roko can be configured to use exponential backoff instead, based on the number of attempts that have occurred so far:

```Go
r := roko.NewRetrier(
  roko.WithMaxAttempts(5),                   // Only try 5 times, then give up
  roko.WithStrategy(roko.Exponential(2, 0)), // Wait (2 ^ attemptCount) + 0 seconds between attempts
)

err := r.Do(func(r *roko.Retrier) error {
  return canFail()
})
```

In this case, the amount of time the retrier will wait between attempts depends on how many attempts have passed - the first wait will be 2^0 == 1 second, then 2^1 == 2 seconds, then 2^3 == 4 seconds, and so on and so forth.

The second argument to the `roko.Exponential()` method is a constant adjustment - roko will add this number to the calculated exponent.

### Using a custom strategy

If the two retry strategies built into roko (`Constant` and `Exponential`) aren't sufficient, you can define your own - the `roko.WithStrategy` method will accept anything that returns a tuple of `(roko.Strategy, string)`. For example, we could implement a custom `Linear` strategy, that multiplies the attempt count by a fixed number:
```Go
func Linear(gradient float64, yIntercept float64) (roko.Strategy, string) {
	return func(r *roko.Retrier) time.Duration {
		return time.Duration(((gradient * float64(r.AttemptCount())) + yIntercept)) * time.Second
	}, "linear" // The second element of the return tuple is the name of the strategy
}

err := roko.NewRetrier(
  roko.WithMaxAttempts(3),             // Only try 3 times, then give up
  roko.WithStrategy(Linear(0.5, 5.0)), // Wait 5 seconds + half of the attempt count seconds
).Do(func(r *roko.Retrier) error {
  return canFail()
})
```

### Manually setting the next interval

Sometimes you only know the desired interval after each try, e.g. a rate-limited API may include a `Retry-After` header. For these cases, the `SetNextInterval(time.Duration)` method can be used. It will apply only to the next interval, and then revert to the configured strategy unless called again on the next attempt.

```Go
// manually specify interval during each try, defaulting to 10 seconds
roko.NewRetrier(
  roko.WithStrategy(Constant(10 * time.Second)),
  roko.WithMaxAttempts(10),
).Do(func(r *roko.Retrier) error {

  response := apiCall() // may be rate limited

  if err := response.HTTPError(); err != nil {
    if response.Status == HttpTooManyRequests {
      if retryAfter, err := strconv.Atoi(response.Header("Retry-After")); err != nil {

        r.SetNextInterval(retryAfter * time.Second) // respect the API

      }
    }
    return err
  }
  return nil
})
```

### Retries and Testing

To speed up tests, roko can be configured with a custom sleep function:

```Go
err := roko.NewRetrier(
  roko.WithStrategy(roko.Constant(50000 * time.Hour)) // Wait a very long time between attempts...
  roko.WithSleepFunc(func(time.Duration) {})          // ...but don't actually sleep
  roko.WithMaxAttempts(3),
).Do(func(r *roko.Retrier) error {
  return canFail()
})
```

The actual function passed to `WithSleepFunc()` is arbitrary, but using a noop is probably going to be the most useful.

For deterministically-generated jitter, the Retrier also accepts a `*rand.Rand`:
```Go
err := roko.NewRetrier(
  roko.WithStrategy(roko.Constant(5 * time.Second))
  roko.WithRand(rand.New(rand.NewSource(12345))), // Generate the same jitters every time, using a seeded random number generator
  roko.WithMaxAttempts(3),
  roko.WithJitter(),
).Do(func(r *roko.Retrier) error {
  return canFail()
})
```

The random number generator is only used for jitter, so it only makes sense to pass one if you're using jitter.

## What's in a name?

Roko is named after [Josevata Rokocoko](https://en.wikipedia.org/wiki/Joe_Rokocoko), a Fijian-New Zealand rugby player, and one of the best to ever do it. He scored a lot of tries, thus, he's a re-trier.

## Contributing

By all means, please contribute! We'd love to have your input. If you run into a bug, feel free to open an issue, and if you find missing functionality, please don't hesitate to open a PR. If you have a weird and wonderful retry strategy you'd like to add, we'd love to see it.

## Looking for a great CI provider? Look no further.

[Buildkite](https://buildkite.com) is a platform for running fast, secure, and scalable CI pipelines on your own infrastructure.
