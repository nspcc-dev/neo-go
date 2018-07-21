# ReadMe 

Currently this package is in Development.

## Convention

There will a util file for every package. These will be utility functions that are only needed in that package.
If utility functions for a package can be modularised further, then you can create a util folder and divide them
again by file. E.g. package/util/pkgutil.go .. package/util/pkgsliceutil.go

The common folder will hold the common utils, that will be used over the whole app.


## References

btcd https://github.com/btcsuite/btcd
geth https://github.com/ethereum/go-ethereum
aeternity https://github.com/aeternity/elixir-node