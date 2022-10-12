package main

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/cmd/benchmarks/db"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	go func() {
		fmt.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	opts := options{
		numSpaces:         1000,
		numEntriesInSpace: 100,
		numChangesInTree:  10,
		numHeadUpdates:    100,
		defValueSize:      1000,
		lenHeadUpdate:     1000,
	}
	fmt.Println("badger")
	bench(db.NewBadgerSpaceCreator, opts)
	fmt.Println("pogreb")
	bench(db.NewPogrebSpaceCreator, opts)
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
	spaceIdKey := func(n int) string {
		return fmt.Sprintf("space%d", n)
	}
	treeIdKey := func(n int) string {
		return fmt.Sprintf("tree%d", n)
	}
	changeIdKey := func(n int) string {
		return fmt.Sprintf("change%d", n)
	}

	var byteSlice = func() []byte {
		var buf = make([]byte, opts.defValueSize)
		rand.Read(buf)
		return buf
	}

	var headUpdate = func() []byte {
		var buf = make([]byte, opts.lenHeadUpdate)
		rand.Read(buf)
		return buf
	}

	creator := factory()
	// creating spaces
	now := time.Now()
	var spaces []db.Space
	for i := 0; i < opts.numSpaces; i++ {
		sp, err := creator.CreateSpace(spaceIdKey(i))
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
		space, err := creator.GetSpace(spaceIdKey(i))
		if err != nil {
			panic(err)
		}
		spaces = append(spaces, space)
		for j := 0; j < opts.numEntriesInSpace; j++ {
			tr, err := space.CreateTree(treeIdKey(j))
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
			err := t.AddChange(changeIdKey(i), byteSlice())
			if err != nil {
				panic(err)
			}
		}
		for i := 0; i < opts.numHeadUpdates; i++ {
			err := t.UpdateHead(string(headUpdate()))
			if err != nil {
				panic(err)
			}
		}
	}
	total := opts.numSpaces * opts.numEntriesInSpace * opts.numChangesInTree
	fmt.Println(total, "changes creation, spent ms", time.Now().Sub(now).Milliseconds())
	now = time.Now()

	// getting some values from tree
	for _, t := range trees {
		for i := 0; i < opts.numChangesInTree; i++ {
			res, err := t.GetChange(changeIdKey(i))
			if err != nil {
				panic(err)
			}
			if res == nil {
				panic("shouldn't be empty")
			}
		}
	}
	fmt.Println(total, "changes getting, spent ms", time.Now().Sub(now).Milliseconds())
	now = time.Now()

	// getting some values from tree
	for _, t := range trees {
		for i := 0; i < opts.numChangesInTree; i++ {
			b, err := t.HasChange(changeIdKey(i))
			if err != nil {
				panic(err)
			}
			if !b {
				panic("should be able to check with has")
			}
		}
	}
	fmt.Println(total, "changes checking, spent ms", time.Now().Sub(now).Milliseconds())

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
