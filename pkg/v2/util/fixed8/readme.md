#Package - Fixed8


## Why

Instead of returning an int64, float64 is now returned. The reasoning behind this was that the Value() function returns the underlying value, which could be 100, or 0.0000001. Instead of returning a string if it is a float, a float64 is returned with enough decimal points to represent the underlying number.

Methods Add and Sub were added for fixed8 arithmetic. Because of floating point inaccuracies, the arithmetic is done with the fixed8 representation. For example, instead of 0.0000001 + 0.0000001 , we do 1 + 1 = 2, then we use the value to initialise a new Fixed8 object.