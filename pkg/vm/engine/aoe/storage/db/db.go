package db

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	e "matrixone/pkg/vm/engine/aoe/storage"
	bmgrif "matrixone/pkg/vm/engine/aoe/storage/buffer/manager/iface"
	"matrixone/pkg/vm/engine/aoe/storage/common"
	"matrixone/pkg/vm/engine/aoe/storage/layout/table"
	"matrixone/pkg/vm/engine/aoe/storage/layout/table/handle"
	ih "matrixone/pkg/vm/engine/aoe/storage/layout/table/handle/base"
	mtif "matrixone/pkg/vm/engine/aoe/storage/memtable/base"
	md "matrixone/pkg/vm/engine/aoe/storage/metadata"
	"matrixone/pkg/vm/engine/aoe/storage/mock/type/chunk"
	mdops "matrixone/pkg/vm/engine/aoe/storage/ops/memdata"
	mops "matrixone/pkg/vm/engine/aoe/storage/ops/meta"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

var (
	ErrClosed = errors.New("aoe: closed")
)

// type Reader interface {
// }
// type Writer interface {
// }

type DB struct {
	Dir  string
	Opts *e.Options

	MemTableMgr     mtif.IManager
	MutableBufMgr   bmgrif.IBufferManager
	TableDataBufMgr bmgrif.IBufferManager

	store struct {
		sync.RWMutex
		MetaInfo   *md.MetaInfo
		DataTables *table.Tables
	}

	DataDir  *os.File
	FileLock io.Closer

	Closed  *atomic.Value
	ClosedC chan struct{}

	sync.RWMutex
}

// var (
// 	_ Reader = (*DB)(nil)
// 	_ Writer = (*DB)(nil)
// )

func cleanStaleMeta(dirname string) {
	dir := e.MakeMetaDir(dirname)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	if len(files) == 0 {
		return
	}

	maxVersion := -1
	maxIdx := -1

	filenames := make(map[int]string)

	for idx, file := range files {
		if e.IsTempFile(file.Name()) {
			log.Infof("Removing %s", path.Join(dir, file.Name()))
			err = os.Remove(path.Join(dir, file.Name()))
			if err != nil {
				panic(err)
			}
		}
		version, ok := e.ParseMetaFileName(file.Name())
		if !ok {
			continue
		}
		if version > maxVersion {
			maxVersion = version
			maxIdx = idx
		}
		filenames[idx] = file.Name()
	}

	if maxIdx == -1 {
		return
	}

	for idx, filename := range filenames {
		if idx == maxIdx {
			continue
		}
		log.Infof("Removing %s", path.Join(dir, filename))
		err = os.Remove(path.Join(dir, filename))
		if err != nil {
			panic(err)
		}
	}
}

func (d *DB) Append(tableName string, ck *chunk.Chunk, index *md.LogIndex) (err error) {
	if err := d.Closed.Load(); err != nil {
		panic(err)
	}
	tbl, err := d.store.MetaInfo.ReferenceTableByName(tableName)
	if err != nil {
		return err
	}

	collection := d.MemTableMgr.GetCollection(tbl.GetID())
	if collection == nil {
		opCtx := &mdops.OpCtx{
			Opts:       d.Opts,
			MTManager:  d.MemTableMgr,
			TableMeta:  tbl,
			BufManager: d.MutableBufMgr,
			Tables:     d.store.DataTables,
		}
		op := mdops.NewCreateTableOp(opCtx)
		op.Push()
		err = op.WaitDone()
		if err != nil {
			panic("logic error")
		}
		collection = op.Collection
	}

	clonedIndex := *index
	return collection.Append(ck, &clonedIndex)
}

func (d *DB) HasTable(name string) bool {
	if err := d.Closed.Load(); err != nil {
		panic(err)
	}
	_, err := d.store.MetaInfo.ReferenceTableByName(name)
	return err == nil
}

func (d *DB) CreateTable(schema *md.Schema) (id uint64, err error) {
	if err := d.Closed.Load(); err != nil {
		panic(err)
	}
	opCtx := &mops.OpCtx{Opts: d.Opts, Schema: schema}
	op := mops.NewCreateTblOp(opCtx)
	op.Push()
	err = op.WaitDone()
	if err != nil {
		return id, err
	}
	id = op.GetTable().GetID()
	return id, nil
}

func (d *DB) NewSegmentIter(o *e.IterOptions) (ih.ISegmentIterator, error) {
	if err := d.Closed.Load(); err != nil {
		panic(err)
	}
	tableMeta, err := d.store.MetaInfo.ReferenceTableByName(o.TableName)
	if err != nil {
		return nil, err
	}
	if tableMeta.GetSegmentCount() == uint64(0) {
		return handle.EmptySegmentIt, nil
	}
	tableData, err := d.store.DataTables.GetTable(tableMeta.ID)
	if err != nil {
		return nil, err
	}
	var h *handle.SegmentsHandle
	if o.All {
		h = handle.NewAllSegmentsHandle(o.ColIdxes, tableData)
	} else {
		h = handle.NewSegmentsHandle(o.SegmentIds, o.ColIdxes, tableData)
	}
	return h.NewSegIt(), nil
}

func (d *DB) NewBlockIter(o *e.IterOptions) (ih.IBlockIterator, error) {
	if err := d.Closed.Load(); err != nil {
		panic(err)
	}
	tableMeta, err := d.store.MetaInfo.ReferenceTableByName(o.TableName)
	if err != nil {
		return nil, err
	}

	if tableMeta.GetSegmentCount() == uint64(0) {
		return handle.EmptyBlockIt, nil
	}

	tableData, err := d.store.DataTables.GetTable(tableMeta.ID)
	if err != nil {
		return nil, err
	}
	var h *handle.SegmentsHandle
	if o.All {
		h = handle.NewAllSegmentsHandle(o.ColIdxes, tableData)
	} else {
		h = handle.NewSegmentsHandle(o.SegmentIds, o.ColIdxes, tableData)
	}
	return h.NewBlkIt(), nil
}

func (d *DB) TableIDs() (ids []uint64, err error) {
	if err := d.Closed.Load(); err != nil {
		panic(err)
	}
	tids := d.store.MetaInfo.TableIDs()
	for tid := range tids {
		ids = append(ids, tid)
	}
	return ids, err
}

func (d *DB) TableSegmentIDs(tableID uint64) (ids []common.ID, err error) {
	if err := d.Closed.Load(); err != nil {
		panic(err)
	}
	sids, err := d.store.MetaInfo.TableSegmentIDs(tableID)
	if err != nil {
		return ids, err
	}
	// TODO: Refactor metainfo to 1. keep order 2. use common.ID
	for sid := range sids {
		ids = append(ids, common.ID{TableID: tableID, SegmentID: sid})
	}
	return ids, err
}

func (d *DB) validateAndCleanStaleData() {
	expectFiles := make(map[string]bool)
	for _, tbl := range d.store.MetaInfo.Tables {
		for _, seg := range tbl.Segments {
			id := common.ID{
				TableID:   seg.TableID,
				SegmentID: seg.ID,
			}
			if seg.DataState == md.SORTED {
				name := e.MakeFilename(d.Dir, e.FTSegment, id.ToSegmentFileName(), false)
				expectFiles[name] = true
			} else {
				for _, blk := range seg.Blocks {
					if blk.DataState == md.EMPTY {
						continue
					}
					id.BlockID = blk.ID
					name := e.MakeFilename(d.Dir, e.FTBlock, id.ToBlockFileName(), false)
					expectFiles[name] = true
				}
			}
		}
	}

	dataDir := e.MakeDataDir(d.Dir)
	if len(expectFiles) == 0 {
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			err = os.MkdirAll(dataDir, 0755)
			if err != nil {
				panic(fmt.Sprintf("err: %s", err))
			}
			return
		}
	}

	err := filepath.Walk(e.MakeDataDir(d.Dir), func(p string, info os.FileInfo, err error) error {
		err = nil
		if e.IsTempFile(info.Name()) {
			log.Infof("Removing %s", p)
			err = os.Remove(p)
			return err
		}
		_, ok := expectFiles[p]
		if !ok {
			log.Infof("Removing %s", p)
			err = os.Remove(p)
		}
		expectFiles[p] = false
		return err
	})
	if err != nil {
		panic(err)
	}

	for name, ok := range expectFiles {
		if ok {
			panic(fmt.Sprintf("Missing %s", name))
		}
	}
}

func (d *DB) startWorkers() {
	d.Opts.MemData.Updater.Start()
	d.Opts.Data.Flusher.Start()
	d.Opts.Data.Sorter.Start()
	d.Opts.Meta.Flusher.Start()
	d.Opts.Meta.Updater.Start()
}

func (d *DB) stopWorkers() {
	d.Opts.MemData.Updater.Stop()
	d.Opts.Data.Flusher.Stop()
	d.Opts.Data.Sorter.Stop()
	d.Opts.Meta.Flusher.Stop()
	d.Opts.Meta.Updater.Stop()
}

func (d *DB) Close() error {
	if err := d.Closed.Load(); err != nil {
		panic(err)
	}

	d.Closed.Store(ErrClosed)
	close(d.ClosedC)
	d.stopWorkers()
	return nil
}