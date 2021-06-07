package data

import (
	e "matrixone/pkg/vm/engine/aoe/storage"
	imem "matrixone/pkg/vm/engine/aoe/storage/memtable/base"
	"matrixone/pkg/vm/engine/aoe/storage/ops"
	iops "matrixone/pkg/vm/engine/aoe/storage/ops/base"
	iworker "matrixone/pkg/vm/engine/aoe/storage/worker/base"
	// log "github.com/sirupsen/logrus"
)

type OpCtx struct {
	MemTable   imem.IMemTable
	Collection imem.ICollection
	Opts       *e.Options
}

type Op struct {
	ops.Op
	Ctx *OpCtx
}

func NewOp(impl iops.IOpInternal, ctx *OpCtx, w iworker.IOpWorker) *Op {
	op := &Op{
		Ctx: ctx,
		Op: ops.Op{
			Impl:   impl,
			ErrorC: make(chan error),
			Worker: w,
		},
	}
	return op
}