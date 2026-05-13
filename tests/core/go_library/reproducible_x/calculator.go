package calculator

import (
	"errors"
)

// Calculator interface defines the basic operations
type Calculator interface {
	Add(a, b float64) float64
	Subtract(a, b float64) float64
	Multiply(a, b float64) float64
	Divide(a, b float64) (float64, error)
}

// SimpleCalculator struct that implements the Calculator interface
type SimpleCalculator struct{}

// Add method
func (c SimpleCalculator) Add(a, b float64) float64 {
	return a + b
}

// Subtract method
func (c SimpleCalculator) Subtract(a, b float64) float64 {
	return a - b
}

// Multiply method
func (c SimpleCalculator) Multiply(a, b float64) float64 {
	return a * b
}

// Divide method (with error handling for division by zero)
func (c SimpleCalculator) Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("division by zero is not allowed")
	}
	return a / b, nil
}
