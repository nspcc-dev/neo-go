package d

func Negate(n int) int {
	n *= -1
	return n
}

func AddNeg(a int, b int) int {
	a *= -1
	b *= -1
	return a + b
}

func Wrap1(n int) int {
	n *= -1
	n *= -1
	return Negate(n)
}

func Wrap2(n int) int {
	n *= -1
	n *= -1
	return Wrap1(n)
}

func SetOneFirstElem(s []int) {
	s[0] = 1
}
