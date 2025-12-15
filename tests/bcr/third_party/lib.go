package third_party

func shadow() int {
	foo := 5
	if foo > 0 {
		// Trigger the "shadow" nogo error.
		foo := 6
		return foo
	}
	return foo
}