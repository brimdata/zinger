package fifo

import (
	"context"
	"fmt"
	"time"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zync/etl"
)

// From syncs data from a Kafka topic to a Zed lake in a
// consistent and crash-recoverable fashion.  The data synced to the lake
// is assigned a target offset in the lake that may be used to then sync
// the merged lake's data back to another Kafka topic using To.
type From struct {
	zctx   *zed.Context
	dst    *Lake
	src    *Consumer
	shaper string
	batch  zbuf.Batch
}

func NewFrom(zctx *zed.Context, dst *Lake, src *Consumer, shaper string) *From {
	return &From{
		zctx:   zctx,
		dst:    dst,
		src:    src,
		shaper: shaper,
	}
}

// Sync syncs data.  If thresh is nonnegative, Sync returns after syncing at
// least thresh records.  Sync also returns if timeout elapses while waiting to
// receive new records from the Kafka topic.
func (f *From) Sync(ctx context.Context, thresh int, timeout time.Duration) (int64, int64, error) {
	offset, err := f.dst.NextProducerOffset(f.src.topic)
	if err != nil {
		return 0, 0, err
	}
	// Loop over the records from the Kafka consumer and
	// commit a batch at a time to the lake.
	var ncommit, nrec int64
	for {
		batch, err := f.src.Read(ctx, thresh, timeout)
		if err != nil {
			return 0, 0, err
		}
		vals := batch.Values()
		n := len(vals)
		if n == 0 {
			break
		}
		batch, err = AdjustOffsetsAndShape(f.zctx, batch, offset, f.shaper)
		if err != nil {
			return 0, 0, err
		}
		//XXX We need to track the commitID and use new commit-only-if
		// constraint and recompute offsets if needed.  See zync issue #16.
		commit, err := f.dst.LoadBatch(f.zctx, batch)
		if err != nil {
			return 0, 0, err
		}
		fmt.Printf("commit %s %d record%s\n", commit, n, plural(n))
		offset += int64(n)
		nrec += int64(n)
		ncommit++
	}
	return ncommit, nrec, nil
}

// AdjustOffsetsAndShape runs a local Zed program to adjust the Kafka offset fields
// for insertion into correct position in the lake and remember the original
// offset along with applying a user-defined shaper.
func AdjustOffsetsAndShape(zctx *zed.Context, batch *zbuf.Array, offset int64, shaper string) (*zbuf.Array, error) {
	vals := batch.Values()
	kafkaRec, err := etl.Field(&vals[0], "kafka")
	if err != nil {
		return nil, err
	}
	first, err := etl.FieldAsInt(kafkaRec, "offset")
	if err != nil {
		return nil, err
	}
	// Send the batch of Zed records through this query to adjust the save
	// the original input offset and adjust the offset so it fits in sequetentially
	// we everything else in the target pool.
	query := fmt.Sprintf("kafka.input_offset:=kafka.offset,kafka.offset:=kafka.offset-%d+%d", first, offset)
	if shaper != "" {
		query = fmt.Sprintf("%s | %s", query, shaper)
	}
	return RunLocalQuery(zctx, batch, query)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
