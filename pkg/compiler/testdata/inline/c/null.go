package c

func Is42(a int) bool {
	if a == 42 {
		return true
	}
	return false
}

func MulIfSmall(n int) int {
	if n < 10 {
		return n * 2
	}
	return n
}

func Transform(a, b int) int {
	if Is42(a) && !Is42(b) {
		return MulIfSmall(b)
	}
	return MulIfSmall(a)
}
