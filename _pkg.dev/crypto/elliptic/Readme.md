## Package - Elliptic 

### Why

The curve and arithmetic functions have been modularised, so that curves can be swapped in and out, without effecting the functionality.

The modular arithmetic used is not specialised for a specific curve.

In order to use this package, you must declare an ellipticcurve struct and then set the curve.

Example:

`

   curve = NewEllipticCurve(Secp256k1)

`
If no curve is set, the default curve is the r1 curve used for NEO. The tests are done using the k1 curve, so in the elliptic_test.go file, the curve is changed accordingly.
