package main

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/cmd/benchmarks/db"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	//bench(db.NewPogrebSpaceCreator, options{
	//	numSpaces:         100,
	//	numEntriesInSpace: 100,
	//	numChangesInTree:  10,
	//	numHeadUpdates:    100,
	//	defValueSize:      1000,
	//	lenHeadUpdate:     10,
	//})
	bench(db.NewPogrebSpaceCreator, options{
		numSpaces:         1000,
		numEntriesInSpace: 100,
		numChangesInTree:  10,
		numHeadUpdates:    100,
		defValueSize:      1000,
		lenHeadUpdate:     10,
	})
}

type options struct {
	numSpaces         int
	numEntriesInSpace int
	numChangesInTree  int
	numHeadUpdates    int
	defValueSize      int
	lenHeadUpdate     int
}

func bench(factory db.SpaceCreatorFactory, opts options) {
	spaceIdGetter := func(n int) string {
		return fmt.Sprintf("space%d", n)
	}
	treeIdGetter := func(n int) string {
		return fmt.Sprintf("tree%d", n)
	}
	changeIdGetter := func(n int) string {
		return fmt.Sprintf("change%d", n)
	}

	var byteSlice []byte
	for i := 0; i < opts.defValueSize; i++ {
		byteSlice = append(byteSlice, byte('a'))
	}

	var headUpdate string
	for i := 0; i < opts.lenHeadUpdate; i++ {
		headUpdate += "a"
	}

	creator := factory()
	// creating spaces
	now := time.Now()
	var spaces []db.Space
	for i := 0; i < opts.numSpaces; i++ {
		sp, err := creator.CreateSpace(spaceIdGetter(i))
		if err != nil {
			panic(err)
		}
		err = sp.Close()
		if err != nil {
			panic(err)
		}
	}
	fmt.Println(opts.numSpaces, "spaces creation, spent ms", time.Now().Sub(now).Milliseconds())
	now = time.Now()
	// creating trees
	var trees []db.Tree
	for i := 0; i < opts.numSpaces; i++ {
		space, err := creator.GetSpace(spaceIdGetter(i))
		if err != nil {
			panic(err)
		}
		spaces = append(spaces, space)
		for j := 0; j < opts.numEntriesInSpace; j++ {
			tr, err := space.CreateTree(treeIdGetter(j))
			if err != nil {
				panic(err)
			}
			trees = append(trees, tr)
		}
	}
	fmt.Println(opts.numSpaces*opts.numEntriesInSpace, "trees creation, spent ms", time.Now().Sub(now).Milliseconds())
	now = time.Now()

	// filling entries and updating heads
	for _, t := range trees {
		for i := 0; i < opts.numChangesInTree; i++ {
			err := t.AddChange(changeIdGetter(i), byteSlice)
			if err != nil {
				panic(err)
			}
		}
		for i := 0; i < opts.numHeadUpdates; i++ {
			err := t.UpdateHead(headUpdate)
			if err != nil {
				panic(err)
			}
		}
	}
	fmt.Println(opts.numSpaces*opts.numEntriesInSpace*opts.numChangesInTree, "changes creation, spent ms", time.Now().Sub(now).Milliseconds())

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-exit
	for _, sp := range spaces {
		err := sp.Close()
		if err != nil {
			panic(err)
		}
	}
	err := creator.Close()
	if err != nil {
		panic(err)
	}
	fmt.Println(sig)
}
