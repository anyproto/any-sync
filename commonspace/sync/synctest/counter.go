package synctest

import (
	"fmt"
	"sync"

	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/util/slice"
)

const CounterName = "counter"

type Counter struct {
	sync.Mutex
	counters     map[int32]struct{}
	peerProvider *PeerProvider
	next, delta  int32
	maxVal       int32
}

func (c *Counter) Init(a *app.App) (err error) {
	c.peerProvider = a.MustComponent(PeerName).(*PeerProvider)
	return nil
}

func (c *Counter) Name() (name string) {
	return CounterName
}

func (c *Counter) Generate() (ret int32) {
	c.Lock()
	defer c.Unlock()
	ret = c.next
	c.next += c.delta
	c.counters[ret] = struct{}{}
	if ret > c.maxVal {
		c.maxVal = ret
	}
	return ret
}

func (c *Counter) CheckComplete() bool {
	c.Lock()
	defer c.Unlock()
	return c.maxVal <= int32(len(c.counters))
}

func (c *Counter) Add(val int32) {
	fmt.Println("adding", val, "peerId", c.peerProvider.myPeer)
	fmt.Println("dumping peerId", c.peerProvider.myPeer, "dump", c.Dump())
	c.Lock()
	defer c.Unlock()
	if val > c.maxVal {
		c.maxVal = val
	}
	c.counters[val] = struct{}{}
}

func (c *Counter) Dump() (ret []int32) {
	c.Lock()
	defer c.Unlock()
	for val := range c.counters {
		ret = append(ret, val)
	}
	slices.Sort(ret)
	return
}

func (c *Counter) DiffCurrentNew(vals []int32) (toSend, toAsk []int32) {
	c.Lock()
	defer c.Unlock()
	m := make(map[int32]struct{})
	for _, val := range vals {
		m[val] = struct{}{}
	}
	_, toSend, toAsk = slice.CompareMaps(c.counters, m)
	return
}

func (c *Counter) KnownCounters() (ret []int32) {
	c.Lock()
	defer c.Unlock()
	for val := range c.counters {
		ret = append(ret, val)
	}
	slices.Sort(ret)
	return
}

func NewCounter(cur, delta int32) *Counter {
	return &Counter{
		counters: make(map[int32]struct{}),
		next:     cur,
		delta:    delta,
	}
}
