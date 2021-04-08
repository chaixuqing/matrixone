package differenceR

import (
	"matrixbase/pkg/container/batch"
	"matrixbase/pkg/hash"
	"matrixbase/pkg/intmap/fastmap"
)

const (
	UnitLimit = 1024
)

var (
	ZeroBools  []bool
	OneUint64s []uint64
)

type Container struct {
	probed  bool
	builded bool
	diffs   []bool
	matchs  []int64
	hashs   []uint64
	sels    [][]int64      // sels
	slots   *fastmap.Map   // hash code -> sels index
	bats    []*batch.Batch // s relation
	state   struct {
		data []byte
		sels []int64
		gs   [][]*hash.BagGroup
	}
	groups map[uint64][]*hash.BagGroup // hash code -> group list
}

type Argument struct {
	R   string
	S   string
	Ctr Container
}