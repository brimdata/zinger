package sync

import (
	"context"
	"errors"
	"flag"
	"fmt"

	lakeapi "github.com/brimdata/zed/lake/api"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/zson"
	"github.com/brimdata/zinger/cli"
	"github.com/brimdata/zinger/fifo"
	"github.com/riferrei/srclient"
)

var FromSpec = &charm.Spec{
	Name:  "from",
	Usage: "from [options]",
	Short: "sync a Kafka topic to a Zed lake pool",
	Long: `
The "from" command syncs data on a Kafka topic to a Zed lake pool.
The Zed records are transcoded from Avro into Zed and synced
to the main branch of the Zed data pool specified.

The data pool's key must be "kafka.offset" sorted in descending order.

See https://github.com/brimdata/zinger/README.md for a description
of how this works.

`,
	New: NewFrom,
}

type From struct {
	*Sync
	group string
	flags cli.Flags
}

func NewFrom(parent charm.Command, fs *flag.FlagSet) (charm.Command, error) {
	f := &From{Sync: parent.(*Sync)}
	fs.StringVar(&f.group, "group", "", "kafka consumer group name")
	f.flags.SetFlags(fs)
	return f, nil
}

//XXX get this working then we need to add Seek() to consumer and start from
// correct position.

func (f *From) Run(args []string) error {
	if f.flags.Topic == "" {
		return errors.New("no topic provided")
	}
	if f.pool == "" {
		return errors.New("no pool provided")

	}
	ctx := context.Background()
	service, err := lakeapi.OpenRemoteLake(ctx, f.flags.Host)
	if err != nil {
		return err
	}
	lk, err := fifo.NewLake(ctx, f.pool, service)
	if err != nil {
		return err
	}
	consumerOffset, err := lk.NextConsumerOffset(f.flags.Topic)
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
	zctx := zson.NewContext()
	consumer, err := fifo.NewConsumer(zctx, config, registry, f.flags.Topic, f.group, consumerOffset, true)
	if err != nil {
		return err
	}
	from := fifo.NewFrom(zctx, lk, consumer)
	ncommit, nrec, err := from.Sync(ctx)
	if ncommit != 0 {
		fmt.Printf("synchronized %d record%s in %d commit%s\n", nrec, plural(nrec), ncommit, plural(ncommit))
	} else {
		fmt.Println("nothing new found to synchronize")
	}
	//XXX close consumer?
	return err
}

func plural(n int64) string {
	if n == 1 {
		return ""
	}
	return "s"
}
