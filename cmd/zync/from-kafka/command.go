package fromkafka

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zync/cli"
	"github.com/brimdata/zync/cmd/zync/root"
	"github.com/brimdata/zync/fifo"
	"github.com/riferrei/srclient"
)

func init() {
	root.Zync.Add(FromSpec)
}

var FromSpec = &charm.Spec{
	Name:  "from-kafka",
	Usage: "from-kafka [options]",
	Short: "sync a Kafka topic to a Zed lake pool",
	Long: `
The "from-kafka" command syncs data on a Kafka topic to a Zed lake pool.
The Zed records are transcoded from Avro into Zed and synced
to the main branch of the Zed data pool specified.

The data pool's key must be "kafka.offset" sorted in ascending order.

See https://github.com/brimdata/zync/README.md for a description
of how this works.

`,
	New: NewFrom,
}

type From struct {
	*root.Command
	flags     cli.Flags
	lakeFlags cli.LakeFlags
	shaper    cli.ShaperFlags
	pool      string
	thresh    int
	timeout   time.Duration
}

func NewFrom(parent charm.Command, fs *flag.FlagSet) (charm.Command, error) {
	f := &From{Command: parent.(*root.Command)}
	fs.StringVar(&f.pool, "pool", "", "name of Zed data pool")
	fs.IntVar(&f.thresh, "thresh", 10*1024*1024, "exit after syncing at least this many records")
	fs.DurationVar(&f.timeout, "timeout", 5*time.Second, "exit after waiting this long for new records")
	f.flags.SetFlags(fs)
	f.lakeFlags.SetFlags(fs)
	f.shaper.SetFlags(fs)
	return f, nil
}

func (f *From) Run(args []string) error {
	if f.flags.Topic == "" {
		return errors.New("no topic provided")
	}
	if f.pool == "" {
		return errors.New("no pool provided")

	}
	shaper, err := f.shaper.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	service, err := f.lakeFlags.Open(ctx)
	if err != nil {
		return err
	}
	lk, err := fifo.NewLake(ctx, f.pool, "", service)
	if err != nil {
		return err
	}
	consumerOffset, err := lk.NextConsumerOffset(ctx, f.flags.Topic)
	if err != nil {
		return err
	}
	url, secret, err := cli.SchemaRegistryEndpoint()
	if err != nil {
		return err
	}
	config, err := cli.LoadKafkaConfig()
	if err != nil {
		return err
	}
	registry := srclient.CreateSchemaRegistryClient(url)
	registry.SetCredentials(secret.User, secret.Password)
	zctx := zed.NewContext()
	consumer, err := fifo.NewConsumer(zctx, config, registry, f.flags.Format, f.flags.Topic, consumerOffset, true)
	if err != nil {
		return err
	}
	from := fifo.NewFrom(zctx, lk, consumer, shaper)
	ncommit, nrec, err := from.Sync(ctx, f.thresh, f.timeout)
	if ncommit != 0 {
		fmt.Printf("synchronized %d record%s in %d commit%s\n", nrec, plural(nrec), ncommit, plural(ncommit))
	} else {
		fmt.Println("nothing new found to synchronize")
	}
	return err
}

func plural(n int64) string {
	if n == 1 {
		return ""
	}
	return "s"
}
