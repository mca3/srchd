package search

func mustInit(driver, name string, config ...map[string]any) Engine {
	e, err := New(driver, name, config...)
	if err != nil {
		panic(err)
	}

	return e
}
