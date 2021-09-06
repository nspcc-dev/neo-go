# Conventions

This document will list conventions that this repo should follow. These are
guidelines and if you believe that one should not be followed, then please state
why in your PR. If you believe that a piece of code does not follow one of the
conventions listed, then please open an issue before making any changes. 

When submitting a new convention, please open an issue for discussion, if
possible please highlight parts in the code where this convention could help the
code readability or simplicity.

## Avoid named return parameters

func example(test int) (num int) {
    a = test + 1
    num = a * test
    return
}

In the above function we have used a named return parameter, which allows you to
include a simple return statement without the variables you are returning. This
practice can cause confusion when functions become large or the logic becomes
complex, so these should be avoided.

## Use error wrapping

Bad:
```
err = SomeAPI()
if err != nil {
    return fmt.Errorf("something bad happened: %v", err)
}
```

Good:
```
err = SomeAPI()
if err != nil {
    return fmt.Errorf("something bad happened: %w", err)
}
```

Error wrapping allows `errors.Is` and `errors.As` usage in upper layer
functions which might be useful.
