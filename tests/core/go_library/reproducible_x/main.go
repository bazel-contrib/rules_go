package main

import (
	"calculator"
	"fmt"
)

func main() {
	// Create an instance of SimpleCalculator
	calculator := calculator.SimpleCalculator{}

	// Perform some operations
	fmt.Println("Add: 5 + 3 =", calculator.Add(5, 3))
	fmt.Println("Subtract: 5 - 3 =", calculator.Subtract(5, 3))
	fmt.Println("Multiply: 5 * 3 =", calculator.Multiply(5, 3))

	result, err := calculator.Divide(5, 0)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Divide: 5 / 3 =", result)
	}

	result, err = calculator.Divide(5, 3)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Divide: 5 / 3 =", result)
	}
}
