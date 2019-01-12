package main

// insertionSort implements the insertion sort algorithm.
func insertionSort(items []data) {
	var n = len(items)
	for i := 1; i < n; i++ {
		j := i
		for j > 0 {
			if items[j-1].commit > items[j].commit {
				items[j-1], items[j] = items[j], items[j-1]
			}
			j = j - 1
		}
	}
}
