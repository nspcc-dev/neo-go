#Package - Fixed8


## Why

Instead of returning an int64, float64 is now returned. The reasoning behind this was that the Value() function returns the underlying value, which could be 100, or 0.0000001. Instead of returning a string if it is a float, a float64 is returned with enough decimal points to represent the underlying number.

Methods Add and Sub were added for fixed8 arithmetic. Because of floating point inaccuracies, the arithmetic is done with the fixed8 representation. For example, instead of 0.0000001 + 0.0000001 , we do 1 + 1 = 2, then we use the value to initialise a new Fixed8 object.


Backwards compatibility:

Instead of util.NewFixed8(9), you can use fixed8.FromInt(9)

Instead of util.DecodeString("23"), you can use fixed8.FromString("23")

As mentioned above, the Value() method now returns a float64, in order to make it backwards compatible, you can cast it to int64.

Example: int64(Fixed8Obj.Value())

There is also an additional method which takes a float64 and converts that to fixed8:

usage: fixed8.FromFloat(34.56778)