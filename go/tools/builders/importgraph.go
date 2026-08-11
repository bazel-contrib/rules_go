package main

// importGraphHasCgo performs a BFS walk of the import graph starting from
// mainArchive. It reads Autolib entries from each .a file's goobj header to
// discover transitive imports. Returns true if "runtime/cgo" is found anywhere
// in the transitive closure.
func importGraphHasCgo(mainArchive string, pkgToFile map[string]string) (bool, error) {
	visited := make(map[string]bool)
	queue := []string{mainArchive}

	for len(queue) > 0 {
		archivePath := queue[0]
		queue = queue[1:]

		if visited[archivePath] {
			continue
		}
		visited[archivePath] = true

		imports, err := readAutolibImports(archivePath)
		if err != nil {
			continue
		}
		for _, imp := range imports {
			if imp == "runtime/cgo" {
				return true, nil
			}
			if filePath, ok := pkgToFile[imp]; ok && !visited[filePath] {
				queue = append(queue, filePath)
			}
		}
	}
	return false, nil
}
