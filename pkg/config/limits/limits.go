/*
Package limits contains a number of system-wide hardcoded constants.
Many of the Neo protocol parameters can be adjusted by the configuration, but
some can not and this package contains hardcoded limits that are relevant for
many applications.
*/
package limits

const (
	// MaxStorageKeyLen is the maximum length of a key for storage items.
	// Contracts can't use keys longer than that in their requests to the DB.
	MaxStorageKeyLen = 64
	// MaxStorageValueLen is the maximum length of a value for storage items.
	// It is set to be the maximum value for uint16, contracts can't put
	// values longer than that into the DB.
	MaxStorageValueLen = 65535
)
