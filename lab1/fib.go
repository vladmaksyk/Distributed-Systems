// +build !solution

package lab1

func fibonacci(n uint) uint {
	var Val1 uint = 0
	var Val2 uint = 1
	var sum uint
	var i uint

	if n == 0 {
		return 0
	} else if n == 1 {
		return 1
	} else {
		for i = 1; i < n; i++ {
			sum = Val1 + Val2
			Val1 = Val2
			Val2 = sum
		}
		return sum
	}
}
