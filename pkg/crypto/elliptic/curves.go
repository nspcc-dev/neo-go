package elliptic

/*
 This file was originally made by vsergeev.

 Modifications have been made under the MIT license.
 License: MIT


*/

import (
	"math/big"
)

var curve Curve

type curveType string

const (
	Secp256r1 curveType = "Secp256r1"
	Secp256k1 curveType = "Secp256k1"
)

func (ChosenCurve *Curve) SetCurveSecp256r1() {
	ChosenCurve.P, _ = new(big.Int).SetString("FFFFFFFF00000001000000000000000000000000FFFFFFFFFFFFFFFFFFFFFFFF", 16) //Q
	ChosenCurve.A, _ = new(big.Int).SetString("FFFFFFFF00000001000000000000000000000000FFFFFFFFFFFFFFFFFFFFFFFC", 16)
	ChosenCurve.B, _ = new(big.Int).SetString("5AC635D8AA3A93E7B3EBBD55769886BC651D06B0CC53B0F63BCE3C3E27D2604B", 16)
	ChosenCurve.G.X, _ = new(big.Int).SetString("6B17D1F2E12C4247F8BCE6E563A440F277037D812DEB33A0F4A13945D898C296", 16)
	ChosenCurve.G.Y, _ = new(big.Int).SetString("4FE342E2FE1A7F9B8EE7EB4A7C0F9E162BCE33576B315ECECBB6406837BF51F5", 16)
	ChosenCurve.N, _ = new(big.Int).SetString("FFFFFFFF00000000FFFFFFFFFFFFFFFFBCE6FAADA7179E84F3B9CAC2FC632551", 16)
	ChosenCurve.H, _ = new(big.Int).SetString("01", 16)
	ChosenCurve.Name = "Secp256r1"
}

func (ChosenCurve *Curve) SetCurveSecp256k1() {
	ChosenCurve.P, _ = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F", 16)
	ChosenCurve.A, _ = new(big.Int).SetString("0000000000000000000000000000000000000000000000000000000000000000", 16)
	ChosenCurve.B, _ = new(big.Int).SetString("0000000000000000000000000000000000000000000000000000000000000007", 16)
	ChosenCurve.G.X, _ = new(big.Int).SetString("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798", 16)
	ChosenCurve.G.Y, _ = new(big.Int).SetString("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8", 16)
	ChosenCurve.N, _ = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	ChosenCurve.H, _ = new(big.Int).SetString("01", 16)
	ChosenCurve.Name = "Secp256k1"
}

func NewEllipticCurve(ct curveType) Curve {
	var curve Curve
	switch ct {
	case Secp256k1:
		curve.SetCurveSecp256k1()
	case Secp256r1:
		curve.SetCurveSecp256r1()
	default:
		curve.SetCurveSecp256r1()
	}
	return curve
}
