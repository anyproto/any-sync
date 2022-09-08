package tree

func commonSnapshotForTwoPaths(ourPath []string, theirPath []string) (string, error) {
	var i int
	var j int
OuterLoop:
	// find starting point from the right
	for i = len(ourPath) - 1; i >= 0; i-- {
		for j = len(theirPath) - 1; j >= 0; j-- {
			// most likely there would be only one comparison, because mostly the snapshot path will start from the root for nodes
			if ourPath[i] == theirPath[j] {
				break OuterLoop
			}
		}
	}
	if i < 0 || j < 0 {
		return "", ErrNoCommonSnapshot
	}
	// find last common element of the sequence moving from right to left
	for i >= 0 && j >= 0 {
		if ourPath[i] == theirPath[j] {
			i--
			j--
		} else {
			break
		}
	}
	return ourPath[i+1], nil
}

func discardFromSlice[T any](elements []T, isDiscarded func(T) bool) []T {
	var (
		finishedIdx = 0
		currentIdx  = 0
	)
	for currentIdx < len(elements) {
		if !isDiscarded(elements[currentIdx]) {
			if finishedIdx != currentIdx {
				elements[finishedIdx] = elements[currentIdx]
			}
			finishedIdx++
		}
		currentIdx++
	}
	elements = elements[:finishedIdx]
	return elements
}
