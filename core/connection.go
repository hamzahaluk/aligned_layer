package connection

import (
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
)

/*
This retry library was inspired by and uses Cenk Alti (https://github.com/cenkalti) backoff library (https://github.com/cenkalti/backoff).
We would like to thank him for his great work.
*/

type PermanentError struct {
	Inner error
}

func (e PermanentError) Error() string { return e.Inner.Error() }
func (e PermanentError) Unwrap() error {
	return e.Inner
}
func (e PermanentError) Is(err error) bool {
	_, ok := err.(PermanentError)
	return ok
}

type TransientError struct {
	Inner error
}

func (e TransientError) Error() string { return e.Inner.Error() }
func (e TransientError) Unwrap() error {
	return e.Inner
}
func (e TransientError) Is(err error) bool {
	_, ok := err.(TransientError)
	return ok
}

const MinDelay = 1000
const RetryFactor = 2
const NumRetries = 3
const MaxInterval = 60000

// Same as Retry only that the functionToRetry can return a value upon correct execution
func RetryWithData[T any](functionToRetry func() (T, error), minDelay uint64, factor float64, maxTries uint64, maxInterval uint64) (T, error) {
	f := func() (T, error) {
		var (
			val T
			err error
		)
		func() {
			defer func() {
				if r := recover(); r != nil {
					if panic_err, ok := r.(error); ok {
						err = TransientError{panic_err}
					} else {
						err = TransientError{fmt.Errorf("panicked: %v", panic_err)}
					}
				}
			}()
			val, err = functionToRetry()
			if perm, ok := err.(PermanentError); err != nil && ok {
				err = backoff.Permanent(perm.Inner)
			}
		}()
		return val, err
	}

	randomOption := backoff.WithRandomizationFactor(0)

	initialRetryOption := backoff.WithInitialInterval(time.Millisecond * time.Duration(minDelay))
	multiplierOption := backoff.WithMultiplier(factor)
	maxIntervalOption := backoff.WithMaxInterval(time.Millisecond * time.Duration(maxInterval))
	expBackoff := backoff.NewExponentialBackOff(randomOption, multiplierOption, initialRetryOption, maxIntervalOption)
	var maxRetriesBackoff backoff.BackOff

	if maxTries > 0 {
		maxRetriesBackoff = backoff.WithMaxRetries(expBackoff, maxTries)
	} else {
		maxRetriesBackoff = expBackoff
	}

	return backoff.RetryWithData(f, maxRetriesBackoff)
}

// Retries a given function in an exponential backoff manner.
// It will retry calling the function while it returns an error, until the max retries.
// If maxTries == 0 then the retry function will run indefinitely until success
// from the configuration are reached, or until a `PermanentError` is returned.
// The function to be retried should return `PermanentError` when the condition for stop retrying
// is met.
func Retry(functionToRetry func() error, minDelay uint64, factor float64, maxTries uint64, maxInterval uint64) error {
	f := func() error {
		err := functionToRetry()
		if perm, ok := err.(PermanentError); err != nil && ok {
			return backoff.Permanent(perm.Inner)
		}
		return err
	}

	randomOption := backoff.WithRandomizationFactor(0)

	initialRetryOption := backoff.WithInitialInterval(time.Millisecond * time.Duration(minDelay))
	multiplierOption := backoff.WithMultiplier(factor)
	maxIntervalOption := backoff.WithMaxInterval(time.Millisecond * time.Duration(maxInterval))
	expBackoff := backoff.NewExponentialBackOff(randomOption, multiplierOption, initialRetryOption, maxIntervalOption)
	var maxRetriesBackoff backoff.BackOff

	if maxTries > 0 {
		maxRetriesBackoff = backoff.WithMaxRetries(expBackoff, maxTries)
	} else {
		maxRetriesBackoff = expBackoff
	}

	return backoff.Retry(f, maxRetriesBackoff)
}
